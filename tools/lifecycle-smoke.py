"""Portable Linux process lifecycle, browser and port checks; synthetic data only."""
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.request

binary = Path('dist/linux-amd64/dayz-bank-viewer').resolve()
with tempfile.TemporaryDirectory(prefix='dayz-lifecycle-') as folder:
    root = Path(folder)
    config = root / 'settings.json'
    processes = []
    def control(action):
        return subprocess.run([binary, '-config', config, '-control', action], capture_output=True, text=True, timeout=20)
    def state():
        for _ in range(100):
            try:
                result = json.loads(Path(str(config) + '.status').read_text())
                urllib.request.urlopen(result['url'] + '/api/version', timeout=1).close()
                return result
            except (OSError, ValueError):
                time.sleep(.05)
        raise AssertionError('service not ready')
    try:
        with socket.socket() as occupied:
            occupied.bind(('127.0.0.1', 0))
            occupied.listen()
            port = occupied.getsockname()[1]
            config.write_text(json.dumps({'listen':f'127.0.0.1:{port}','servers':[],'refresh_times':[]}))
            proc = subprocess.Popen([binary, '-config', config, '-headless'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            processes.append(proc)
            initial = state()
            assert port < int(initial['url'].rsplit(':',1)[1]) <= port + 20
            assert 'token' not in initial or not initial['token']
            duplicate = subprocess.run([binary, '-config', config, '-headless'], capture_output=True, text=True, timeout=5)
            assert duplicate.returncode != 0 and 'already running' in duplicate.stderr
            assert control('start').returncode == 0
            assert state()['control'] == initial['control']
            assert control('restart').returncode == 0
            assert state()['control'] != initial['control']
            assert control('stop').returncode == 0
            proc.wait(timeout=5)
            assert not Path(str(config)+'.runtime').exists()
        # Native desktop tools are stubbed to check launch policy without opening real windows.
        bin_dir = root / 'bin'
        bin_dir.mkdir()
        marker = root / 'browser-opens'
        (bin_dir / 'xdg-open').write_text('#!/bin/sh\nprintf "%s\\n" "$1" >> "$BROWSER_MARKER"\n')
        (bin_dir / 'zenity').write_text('#!/bin/sh\nexit 1\n')
        for tool in bin_dir.iterdir():
            tool.chmod(0o755)
        env = dict(os.environ, PATH=str(bin_dir)+':'+os.environ['PATH'], BROWSER_MARKER=str(marker))
        launches = [subprocess.Popen([binary, '-config', config, '-desktop'], env=env) for _ in range(2)]
        for launch in launches:
            assert launch.wait(timeout=20) == 0
        subprocess.run([binary, '-config', config, '-desktop'], env=env, check=True, timeout=20)
        assert marker.read_text().splitlines() == [state()['url']]
        assert control('stop').returncode == 0
        assert config.exists()
        print('Lifecycle passed: socket fallback, duplicate refusal, reconnect, restart, stop, readiness, single browser launch, config retained.')
    finally:
        control('stop')
        for proc in processes:
            if proc.poll() is None:
                proc.terminate()
                proc.wait(timeout=5)
