#!/usr/bin/env python3
"""Exercise an isolated, non-root development container and remove its own fixtures."""
import http.cookiejar
import io
import json
import re
import subprocess
import tarfile
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


def docker(*args, input=None):
    result = subprocess.run(['docker', *args], input=input, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(result.stderr or result.stdout)
    return result.stdout.strip()


def main():
    name = 'folkweave-test-' + uuid.uuid4().hex
    docker('volume', 'create', '--label', 'org.comfyware.fixture=true', name)
    started = False
    try:
        # Inspect the image directory before Docker copies it into a named volume.
        image_container = name + '-image'
        docker('create', '--name', image_container, 'folkweave:dev')
        try:
            archive = subprocess.run(['docker', 'export', image_container],
                                     capture_output=True, check=True)
            with tarfile.open(fileobj=io.BytesIO(archive.stdout)) as rootfs:
                directory = rootfs.getmember('data')
                assert directory.isdir()
                assert directory.mode == 0o700, oct(directory.mode)
                assert (directory.uid, directory.gid) == (65532, 65532)
        finally:
            subprocess.run(['docker', 'rm', '-v', image_container], capture_output=True)
        docker('run', '--rm', '-i', '--mount', f'type=volume,source={name},target=/data',
               'folkweave:dev', 'init', '--data', '/data', '--username', 'owner',
               '--name', 'Owner', '--title', 'Container folks', '--password-stdin',
               input='a-private-test-password\n')
        docker('run', '-d', '--name', name, '--label', 'org.comfyware.fixture=true',
               '--read-only', '--cap-drop=ALL', '--security-opt=no-new-privileges:true',
               '--tmpfs', '/tmp:rw,noexec,nosuid,size=128m,mode=1777',
               '--mount', f'type=volume,source={name},target=/data',
               '-e', 'FOLKWEAVE_BACKUP_INTERVAL=0', '-p', '127.0.0.1::8084', 'folkweave:dev')
        started = True
        info = json.loads(docker('inspect', name))[0]
        assert info['Config']['User'] == '65532:65532'
        assert info['HostConfig']['ReadonlyRootfs']
        assert 'ALL' in info['HostConfig']['CapDrop']
        port = info['NetworkSettings']['Ports']['8084/tcp'][0]['HostPort']
        base = 'http://127.0.0.1:' + port
        client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

        def request(path, form=None):
            data = urllib.parse.urlencode(form).encode() if form is not None else None
            req = urllib.request.Request(base + path, data=data, headers={'Host': 'localhost:8084'})
            with client.open(req, timeout=5) as response:
                return response.status, response.read().decode()

        for attempt in range(50):
            try:
                status, body = request('/healthz')
                if status == 200:
                    break
            except (OSError, urllib.error.URLError):
                time.sleep(.1)
        else:
            raise RuntimeError('container did not become ready: ' + docker('logs', name))
        status, body = request('/login')
        csrf = re.search(r'name="csrf" value="([a-f0-9]+)"', body).group(1)
        status, body = request('/login', {'username': 'owner', 'password': 'a-private-test-password', 'csrf': csrf})
        assert status == 200 and 'Container folks' in body
        docker('exec', name, '/usr/local/bin/folkweave', 'backup', '--data', '/data', '--output', '/data/smoke-snapshot')
        docker('stop', '--time', '15', name)
        assert json.loads(docker('inspect', name))[0]['State']['ExitCode'] == 0
        print('PASS: non-root, read-only container; private volume; login; live verified snapshot; graceful shutdown')
    finally:
        if started:
            subprocess.run(['docker', 'rm', '-f', name], capture_output=True)
        subprocess.run(['docker', 'volume', 'rm', name], capture_output=True)


if __name__ == '__main__':
    main()
