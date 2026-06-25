#!/usr/bin/env python3
"""Post-process swagger.json: merge BearerAuth + ProjectKey into AND security."""
import json, sys

if len(sys.argv) < 2:
    print("Usage: fix-swagger-security.py <swagger.json>")
    sys.exit(1)

with open(sys.argv[1]) as f:
    spec = json.load(f)

fixed = 0
for path, methods in spec['paths'].items():
    for method, details in methods.items():
        sec = details.get('security', [])
        if not sec or len(sec) < 2:
            continue
        has_bearer = any('BearerAuth' in s for s in sec)
        has_project = any('ProjectKey' in s for s in sec)
        if has_bearer and has_project and len(sec) == 2:
            details['security'] = [{'BearerAuth': [], 'ProjectKey': []}]
            fixed += 1

with open(sys.argv[1], 'w') as f:
    json.dump(spec, f, indent=4)
    f.write('\n')

print(f'Fixed {fixed} endpoints: ProjectKey OR BearerAuth → ProjectKey AND BearerAuth')
