# Security

Hecatron runs installers that change your machine, some with sudo, and runs
the build and test commands in your repository. If you find a way for it to
do something it shouldn't (run code from somewhere unexpected, download
without checking what it got, change files outside what it says it does),
please report it privately rather than in a public issue: use **Report a
vulnerability** on the repository's
[Security tab](https://github.com/dnf/hecatron/security).

Fixes go into the next release; there are no separately supported older
versions before 1.0.
