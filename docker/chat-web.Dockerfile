# 只发布独立聊天页的静态白名单，不暴露旧用户数据。
FROM nginx:stable-alpine
COPY docker/chat-nginx.conf /etc/nginx/conf.d/default.conf
COPY public/go-chat.html /usr/share/nginx/html/index.html
COPY public/css/go-chat.css public/css/fontawesome.min.css public/css/solid.min.css /usr/share/nginx/html/css/
COPY public/webfonts/ /usr/share/nginx/html/webfonts/
COPY public/img/ai4.png /usr/share/nginx/html/img/ai4.png
COPY public/scripts/api-client.js public/scripts/api-events.js /usr/share/nginx/html/scripts/
COPY public/scripts/go-chat/ /usr/share/nginx/html/scripts/go-chat/
