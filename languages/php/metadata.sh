#!/usr/bin/env bash
# Update PHP package metadata
source "$(dirname "$0")/../_common.sh"

python3 -c "
import json, os
f = os.path.join('${ROOT}', 'age', 'impl', 'php', 'composer.json')
with open(f) as fh: data = json.load(fh)
data['description'] = os.environ['PT_DESCRIPTION']
data['license'] = os.environ['PT_LICENSE']
data['authors'] = [{'name': os.environ['PT_AUTHOR']}]
data['homepage'] = os.environ['PT_REPOSITORY']
data['support']['source'] = os.environ['PT_REPOSITORY'] + '/tree/main/age/impl/php'
data['support']['issues'] = os.environ['PT_REPOSITORY'] + '/issues'
with open(f, 'w') as fh:
    json.dump(data, fh, indent=4, ensure_ascii=False)
    fh.write('\n')
"
