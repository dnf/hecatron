- **A git worktree of hecatron runs its own binary:** `bin/hecatron` read
  `.git/HEAD`, which a worktree doesn't have (its `.git` is a file naming
  the git directory), so after `hct self build` in a worktree, `hct status`
  and the other binary-only commands still said they need the binary.
