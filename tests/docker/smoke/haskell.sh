# Smoke test for haskell: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'main :: IO ()\nmain = putStrLn "hello"\n' > Hello.hs
ghc -Wall -Werror -o hello Hello.hs && ./hello
cabal --version
