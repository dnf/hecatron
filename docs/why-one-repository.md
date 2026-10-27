# Why one repository

Historically, a library has existed in one language, or at most a handful.
That was never a judgement about where the library belonged; it was a cost.
Porting a library takes time, and keeping each port in sync with the
original takes time again with every release, so most libraries never get
ported, and the ports that do exist are written by other people and lag
behind. Bindings to a native library fill some gaps, but they're awkward to
install and feel foreign in the language that calls them.

So learning a new language has meant learning a new ecosystem of libraries
too. The date library, the parser, the API client you rely on in one
language usually isn't there in the next, and what's there instead works
differently. A useful library ought to exist in every language its users
write; until now, writing and maintaining a native, idiomatic
implementation per language needed a team fluent in each one, so almost
nobody did it.

That doesn't have to be the case any more. AI coding agents change the cost. An agent can write, port and maintain an
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
