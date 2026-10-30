"""Inspect generated packages without installing services on the development host."""
from pathlib import Path
import hashlib
import subprocess
import tarfile
import tempfile
import zipfile

version = Path('VERSION').read_text().strip()
allowed = {'dayz-bank-viewer', 'dayz-bank-viewer.exe', 'README.md', 'config.example.json', 'VERSION', 'docs/TESTING.md', 'LICENSING.md', 'LICENSE', 'web/app.ico', 'web/app.png', 'GO-LICENSE', 'GO-PATENTS'}
for suffix in ['linux-amd64.tar.gz', 'windows-amd64.zip']:
    file = Path(f'dist/dayz-bank-viewer-{version}-{suffix}')
    if suffix.endswith('.zip'):
        with zipfile.ZipFile(file) as archive:
            names = archive.namelist()
    else:
        with tarfile.open(file) as archive:
            names = archive.getnames()
    members = {'/'.join(name.split('/')[1:]) for name in names}
    assert members <= allowed
    assert len(members) == (11 if suffix.endswith('.zip') else 10), members
    if suffix.endswith('.zip'):
        with zipfile.ZipFile(file) as archive:
            logo = next(name for name in names if name.endswith('/web/app.png'))
            assert archive.read(logo) == Path('web/app.png').read_bytes()
    assert 'LICENSE' in members and 'GO-LICENSE' in members and 'GO-PATENTS' in members
package = Path(f'dist/dayz-bank-viewer-{version}-linux-amd64.deb')
with tempfile.TemporaryDirectory() as temp:
    root = Path(temp)
    subprocess.run(['dpkg-deb', '-x', package, root], check=True)
    subprocess.run(['dpkg-deb', '-e', package, root / 'DEBIAN'], check=True)
    for script in ['postinst', 'prerm', 'postrm']:
        file = root / 'DEBIAN' / script
        assert file.stat().st_mode & 0o111
        subprocess.run(['sh', '-n', file], check=True)
    service = (root / 'lib/systemd/system/dayz-bank-viewer.service').read_text()
    assert 'User=dayz-bank-viewer' in service and 'Restart=on-failure' in service
    assert '-service -config' in service
    assert (root / 'usr/bin/dayz-bank-viewer').stat().st_mode & 0o111
    assert not (root / 'var/lib/dayz-bank-viewer/config.json').exists()
setup = Path(f'dist/dayz-bank-viewer-{version}-windows-amd64-setup.exe')
assert setup.read_bytes()[:2] == b'MZ'
for line in Path('dist/SHA256SUMS').read_text().splitlines():
    digest, name = line.split('  ', 1)
    assert hashlib.sha256((Path('dist') / name).read_bytes()).hexdigest() == digest, name
print('Package inspection passed: archive allowlists, licence notices, Debian scripts, service identity, executable modes, Windows PE installer and all checksums.')
