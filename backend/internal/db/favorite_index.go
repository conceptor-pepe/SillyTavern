// favorite_index.go 校验并修复历史 GORM 收藏索引，不删除或重写任何收藏记录。
package db

import (
	"errors"
	"slices"
	"strings"

	"gorm.io/gorm"
)

// indexPart 读取真实索引列序和唯一性，不能仅凭索引名称判断升级成功。
type indexPart struct {
	IndexName  string
	ColumnName string
	NonUnique  int
}

// checkLegacy 必须在 AutoMigrate 前检查已知索引，避免 GORM 先改掉未知约束掩盖漂移。
func checkLegacy(conn *gorm.DB) error {
	parts, err := favoriteIndexes(conn)
	if err != nil {
		return err
	}
	_, err = favoriteChanges(parts)
	return err
}

// favoriteIndexes 只查询当前数据库收藏表的两个已知索引名。
func favoriteIndexes(conn *gorm.DB) ([]indexPart, error) {
	var parts []indexPart
	err := conn.Raw(`SELECT index_name, column_name, non_unique FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'favorites'
		AND index_name IN ('uk_user_char_kind', 'uk_favorites_user_char_kind')
		ORDER BY index_name, seq_in_index`).Scan(&parts).Error
	return parts, err
}

// indexColumns 拒绝同名非唯一索引，避免将未知结构误当成已知历史版本修复。
func indexColumns(parts []indexPart, name string) ([]string, error) {
	var columns []string
	for _, part := range parts {
		if part.IndexName != name {
			continue
		}
		if part.NonUnique != 0 {
			return nil, errors.New("favorite index is not unique")
		}
		columns = append(columns, part.ColumnName)
	}
	return columns, nil
}

// fixFavorites 先保证正确唯一键，再原子移除旧约束；不支持的结构直接阻止启动。
func fixFavorites(conn *gorm.DB) error {
	parts, err := favoriteIndexes(conn)
	if err != nil {
		return err
	}
	changes, err := favoriteChanges(parts)
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		if err := conn.Exec("ALTER TABLE favorites " + strings.Join(changes, ", ")).Error; err != nil {
			return err
		}
	}
	if err := checkFavorites(conn); err != nil {
		return err
	}
	return markChange(conn)
}

// checkFavorites 已完成版本再次启动只接受目标结构，结构漂移需要人工审查。
func checkFavorites(conn *gorm.DB) error {
	parts, err := favoriteIndexes(conn)
	if err != nil {
		return err
	}
	changes, err := favoriteChanges(parts)
	if err != nil || len(changes) != 0 {
		return errors.Join(errors.New("favorite index verification failed"), err)
	}
	return nil
}

// favoriteChanges 仅生成固定标识符的 DDL，不拼接来自数据库的任意名称。
func favoriteChanges(parts []indexPart) ([]string, error) {
	current, err := indexColumns(parts, "uk_favorites_user_char_kind")
	if err != nil {
		return nil, err
	}
	old, err := indexColumns(parts, "uk_user_char_kind")
	if err != nil {
		return nil, err
	}
	var changes []string
	if len(current) == 0 {
		changes = append(changes, "ADD UNIQUE KEY uk_favorites_user_char_kind (user_id, character_id, kind)")
	} else if !slices.Equal(current, []string{"user_id", "character_id", "kind"}) {
		return nil, errors.New("unsupported favorite index")
	}
	if len(old) == 0 {
		return changes, nil
	}
	if !slices.Equal(old, []string{"character_id", "kind"}) {
		return nil, errors.New("unsupported legacy favorite index")
	}
	return append(changes, "DROP INDEX uk_user_char_kind"), nil
}
