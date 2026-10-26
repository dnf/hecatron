#!/usr/bin/env bash
# Update TypeScript package metadata
source "$(dirname "$0")/../_common.sh"

python3 -c "
import json, os
f = os.path.join('${ROOT}', 'age', 'impl', 'typescript', 'package.json')
with open(f) as fh: data = json.load(fh)
data['description'] = os.environ['PT_DESCRIPTION']
data['author'] = os.environ['PT_AUTHOR']
data['license'] = os.environ['PT_LICENSE']
data['repository']['url'] = os.environ['PT_REPOSITORY'] + '.git'
data['homepage'] = os.environ['PT_REPOSITORY']
data['bugs'] = os.environ['PT_REPOSITORY'] + '/issues'
with open(f, 'w') as fh:
    json.dump(data, fh, indent=2, ensure_ascii=False)
    fh.write('\n')
"
