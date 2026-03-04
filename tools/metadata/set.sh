#!/usr/bin/env bash
# Interactive metadata updater for all language manifests.
# Usage: bash .hecatron/tools/metadata/set.sh
# Called by: just metadata
set -euo pipefail

if [ ! -t 0 ]; then
    echo "error: metadata set requires an interactive terminal" >&2
    exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
source "${ROOT}/.hecatron/tools/language/helpers.sh"

METADATA="${ROOT}/sayable-age/config/metadata.json"
if [ ! -f "${METADATA}" ]; then
    echo "error: config/metadata.json not found" >&2
    exit 1
fi

# Read current values
cur_name="$(python3 -c "import json; print(json.load(open('${METADATA}'))['name'])")"
cur_description="$(python3 -c "import json; print(json.load(open('${METADATA}'))['description'])")"
cur_author="$(python3 -c "import json; print(json.load(open('${METADATA}'))['author'])")"
cur_license="$(python3 -c "import json; print(json.load(open('${METADATA}'))['license'])")"
cur_repository="$(python3 -c "import json; print(json.load(open('${METADATA}'))['repository'])")"

echo "Current metadata:"
echo "  name:        ${cur_name}"
echo "  description: ${cur_description}"
echo "  author:      ${cur_author}"
echo "  license:     ${cur_license}"
echo "  repository:  ${cur_repository}"
echo ""

# Prompt for each field (Enter to keep current value)
printf "name [%s]: " "${cur_name}"
read -r new_name
new_name="${new_name:-${cur_name}}"

printf "description [%s]: " "${cur_description}"
read -r new_description
new_description="${new_description:-${cur_description}}"

printf "author [%s]: " "${cur_author}"
read -r new_author
new_author="${new_author:-${cur_author}}"

printf "license [%s]: " "${cur_license}"
read -r new_license
new_license="${new_license:-${cur_license}}"

printf "repository [%s]: " "${cur_repository}"
read -r new_repository
new_repository="${new_repository:-${cur_repository}}"

# Check for changes
if [ "${new_name}" = "${cur_name}" ] && \
   [ "${new_description}" = "${cur_description}" ] && \
   [ "${new_author}" = "${cur_author}" ] && \
   [ "${new_license}" = "${cur_license}" ] && \
   [ "${new_repository}" = "${cur_repository}" ]; then
    echo ""
    echo "No changes."
    exit 0
fi

echo ""
echo "Updating metadata..."

# Write updated config/metadata.json
python3 -c "
import json
data = {
    'name': '''${new_name}''',
    'description': '''${new_description}''',
    'author': '''${new_author}''',
    'license': '''${new_license}''',
    'repository': '''${new_repository}'''
}
with open('${METADATA}', 'w') as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write('\n')
"
echo "  updated config/metadata.json"

# Export both old and new values for installer scripts
export PT_OLD_NAME="${cur_name}"
export PT_OLD_DESCRIPTION="${cur_description}"
export PT_OLD_AUTHOR="${cur_author}"
export PT_OLD_LICENSE="${cur_license}"
export PT_OLD_REPOSITORY="${cur_repository}"

export PT_NAME="${new_name}"
export PT_DESCRIPTION="${new_description}"
export PT_AUTHOR="${new_author}"
export PT_LICENSE="${new_license}"
export PT_REPOSITORY="${new_repository}"

# Update all language implementations
for lang in ${_ALL_LANGS}; do
    [ -z "${lang}" ] && continue
    if [ -f "${ROOT}/.hecatron/languages/${lang}/metadata.sh" ]; then
        bash "${ROOT}/.hecatron/languages/${lang}/metadata.sh"
        echo "  updated ${lang}"
    fi
done

echo ""
echo "Done. Metadata updated across all manifests."
echo "Run 'just test' to verify, then commit."
