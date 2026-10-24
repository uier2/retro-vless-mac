#!/usr/bin/env python3
"""Build an unsigned Intel macOS app on Linux or macOS (Python is build-only)."""
import hashlib
import os
from pathlib import Path
import plistlib
import re
import shutil
import subprocess
import tarfile
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parent
VERSION = '1.3.6'
CORE_URL = 'https://github.com/SagerNet/sing-box/releases/download/v1.3.6/sing-box-1.3.6-darwin-amd64.tar.gz'
# Calculated from the actual official release archive, not a reconstructed binary.
CORE_SHA256 = 'cc7599906b1379c334c9af6f4ce7054e52f43c3d7059ef3a7509523325a7601d'

def main():
    go = os.environ.get('GO') or shutil.which('go')
    if not go:
        raise SystemExit('Install Go 1.20.x or set GO to its executable path.')
    version = subprocess.check_output([go, 'version'], text=True).strip()
    if not re.search(r'\bgo1\.20(?:\.\d+)?\s', version):
        raise SystemExit('Go 1.20.x is mandatory for the macOS 10.13 target; found: ' + version)
    build = ROOT / 'build'
    build.mkdir(exist_ok=True)
    archive = build / 'sing-box-1.3.6-darwin-amd64.tar.gz'
    if not archive.exists():
        temp = archive.with_suffix('.download')
        urllib.request.urlretrieve(CORE_URL, temp)
        temp.replace(archive)
    actual = hashlib.sha256(archive.read_bytes()).hexdigest()
    if actual != CORE_SHA256:
        raise SystemExit('Core archive SHA256 mismatch; remove the cached archive and retry.')
    dist = ROOT / 'dist'
    dist.mkdir(exist_ok=True)
    app = dist / 'Retro VLESS.app'
    if app.exists():
        shutil.rmtree(app)
    mac = app / 'Contents' / 'MacOS'
    res = app / 'Contents' / 'Resources'
    mac.mkdir(parents=True)
    res.mkdir(parents=True)
    prefix = 'sing-box-1.3.6-darwin-amd64/'
    with tarfile.open(archive, 'r:gz') as tar:
        for name, target in [('sing-box', mac / 'sing-box'), ('LICENSE', res / 'sing-box-LICENSE.txt')]:
            member = tar.getmember(prefix + name)
            if not member.isfile():
                raise SystemExit('Unexpected archive member: ' + name)
            with tar.extractfile(member) as source:
                target.write_bytes(source.read())
    env = dict(os.environ, CGO_ENABLED='0', GOOS='darwin', GOARCH='amd64', GOAMD64='v1')
    subprocess.run([go, 'build', '-trimpath', '-ldflags=-s -w', '-o', str(mac / 'retro-vless'), '.'], cwd=ROOT, env=env, check=True)
    shutil.copy2(ROOT / 'launch', mac / 'launch')
    shutil.copy2(ROOT / 'ui.applescript', res / 'ui.applescript')
    shutil.copy2(ROOT / 'LICENSE', res / 'LICENSE')
    for f in mac.iterdir():
        f.chmod(0o755)
    info = {'CFBundleName': 'Retro VLESS', 'CFBundleDisplayName': 'Retro VLESS', 'CFBundleIdentifier': 'org.retrovless.legacy', 'CFBundleVersion': '2', 'CFBundleShortVersionString': '1.0.1', 'CFBundlePackageType': 'APPL', 'CFBundleExecutable': 'launch', 'LSMinimumSystemVersion': '10.13.6', 'NSHighResolutionCapable': True}
    (app / 'Contents' / 'Info.plist').write_bytes(plistlib.dumps(info))
    source = dist / 'source'
    if source.exists():
        shutil.rmtree(source)
    source.mkdir()
    # Explicit public-source allowlist: never bundle user data or arbitrary files.
    files = [f for f in ROOT.iterdir() if f.suffix in ('.go', '.applescript') or f.name in ('go.mod', 'launch', 'build.py', 'README.md', 'LICENSE', '.gitignore')]
    for f in files:
        shutil.copy2(f, source / f.name)
    for name in ('README.md', 'LICENSE'):
        shutil.copy2(ROOT / name, dist / name)
    (dist / 'SOURCES.txt').write_text('Retro VLESS is an independent wrapper, not an upstream sing-box fork.\nBundled core: official sing-box v1.3.6 (GPL-3.0-or-later).\nRelease: https://github.com/SagerNet/sing-box/releases/tag/v1.3.6\nCorresponding upstream source: https://github.com/SagerNet/sing-box/tree/v1.3.6\nSource archive: https://github.com/SagerNet/sing-box/archive/refs/tags/v1.3.6.tar.gz\nCore archive SHA256: ' + CORE_SHA256 + '\nWrapper toolchain: ' + version + '\nWrapper: CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 GOAMD64=v1.\nWrapper source and build script are in source/.\nHigh Sierra runtime has not been verified.\n', encoding='utf-8')
    output = dist / 'Retro-VLESS-Mac-Intel.zip'
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as z:
        for folder in (app, source):
            for f in sorted(folder.rglob('*')):
                if f.is_file():
                    z.write(f, f.relative_to(dist))
        for name in ('README.md', 'LICENSE', 'SOURCES.txt'):
            z.write(dist / name, name)
    digest = hashlib.sha256(output.read_bytes()).hexdigest()
    (dist / 'SHA256.txt').write_text(digest + '  ' + output.name + '\n')
    print(output)
    print('SHA256:', digest)
    print('Bytes:', output.stat().st_size)

if __name__ == '__main__':
    main()
