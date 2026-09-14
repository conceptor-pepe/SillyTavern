import { spawn } from 'node:child_process';
import { openSync, closeSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { setTimeout } from 'node:timers/promises';

const cwd = fileURLToPath(new URL('../', import.meta.url));
const url = 'http://127.0.0.1:8000/';
const log = '/tmp/ai-chat-8000.log';
async function ready() {
    try {
        return (await fetch(`${url}csrf-token`, { signal: AbortSignal.timeout(1000) })).ok;
    } catch {
        return false;
    }
}

if (await ready()) {
    console.log(`Service already available: ${url}`);
} else {
    const fd = openSync(log, 'a', 0o600);
    const child = spawn(process.execPath, ['server.js', '--port', '8000'], {
        cwd, detached: true, stdio: ['ignore', fd, fd],
    });
    child.on('error', error => {
        console.error(error.message);
        process.exitCode = 1;
    });
    child.unref();
    closeSync(fd);
    let started = false;
    for (let attempt = 0; attempt < 30; attempt++) {
        await setTimeout(1000);
        if (await ready()) {
            started = true;
            break;
        }
    }
    if (!started) throw new Error(`Startup failed. Inspect ${log}`);
    console.log(`Service available: ${url} (PID ${child.pid}, log ${log})`);
}
