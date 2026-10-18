"""Translate only explicit control-container mounts for the host Docker daemon."""
import json
import os
from pathlib import PurePosixPath


def rewrite(arguments, encoded_mounts):
    mounts = json.loads(encoded_mounts)
    if not isinstance(mounts, list):
        raise ValueError('invalid mount inventory')
    mappings = sorted(mounts, key=lambda item: len(item['Destination']), reverse=True)
    result = list(arguments)
    # Only the Worker uses this shim. Its task containers share the image,
    # but must not inherit health probes for the separate control processes.
    if result and result[0] in ('create', 'run'):
        result.insert(1, '--no-healthcheck')
    for index, argument in enumerate(result):
        if index == 0 or result[index - 1] != '--mount':
            continue
        fields = argument.split(',')
        if 'type=bind' not in fields:
            continue
        for part, field in enumerate(fields):
            key, _, source = field.partition('=')
            if key not in ('src', 'source'):
                continue
            if not source.startswith('/') or '..' in PurePosixPath(source).parts:
                raise ValueError('bind source must be an absolute confined path')
            resolved = os.path.realpath(source)
            for mount in mappings:
                target, host = mount['Destination'].rstrip('/'), mount['Source'].rstrip('/')
                if target in ('', '/var/run/docker.sock') or not host.startswith('/') or ',' in host:
                    continue
                confined = source.startswith(target + '/') and resolved.startswith(target + '/')
                exact_readonly_file = source == target and resolved == target and not mount.get('RW', True)
                if confined or exact_readonly_file:
                    fields[part] = key + '=' + host + source[len(target):]
                    break
            else:
                raise ValueError('bind source is outside the explicitly shared data paths')
        result[index] = ','.join(fields)
    return result
