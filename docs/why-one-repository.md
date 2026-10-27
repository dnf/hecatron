# Why one repository

A useful library ought to exist in every language its users write. Usually it
doesn't: there's a reference implementation in one language, ports by other
people that lag behind it, and bindings to a native library that are awkward
to install and feel foreign in the language that calls them. Writing and
maintaining a native, idiomatic implementation per language used to need a
team fluent in each one, so almost nobody did it.

AI coding agents change that cost. An agent can write, port and maintain an
implementation in a language its human maintainer doesn't know, so one person
can ship the same library natively in a dozen or fifty languages. Once
implementations are cheap, the best place for them is **one repository**:

- **One change lands everywhere.** A bug fix or a new feature is one pull
  request across every language, not fifty, and no implementation drifts.
- **One specification and one test suite.** Shared test data defines the
  behaviour, and every language's tests run against it.
- **Agents see everything.** An agent porting a change reads the
  implementation it's porting from, in the same checkout.
- **Users get native packages.** Each language publishes to its own registry
  (crates.io, PyPI, npm, Maven Central...) with an API that feels native, not
  a wrapper around someone else's runtime.

What's left is the infrastructure, and it grows with every language. Each one
has a toolchain to install, and toolchains break in their own ways: a linter
built with an older compiler, a virtualenv that moved, a Homebrew formula
left unlinked. Each has its own test, lint and build commands, CI jobs and
Dockerfile lines. With fifty languages, keeping that working becomes the job.

Hecatron does that job. It grew up inside
[sayable](https://github.com/dnf/sayable), a set of human-friendly formatting
libraries implemented in 51 languages, and was split out so any polyglot
repository can use it.

[Back to the README](../README.md)
