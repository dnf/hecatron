- **Fixed: a random "Unknown language", skipped dependencies or wrong doctor
  result under load.** The shell scripts checked list membership by piping
  into `grep -q`; when grep matched early, the writer was killed and, under
  `pipefail`, the check said "not found". They now use a plain loop (#197).
