#!/usr/bin/env bash
# Discard stdin (pre-rendered content) and output a fixed Namespace resource.
# This verifies that the post-renderer output — not the original templates — is
# written to the output directory.
cat > /dev/null
printf -- "---\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: postrendered\n"
