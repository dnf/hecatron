module Main (main) where

import Add (add)
import System.Exit (exitFailure)

main :: IO ()
main
  | add 2 3 == 5 = putStrLn "ok"
  | otherwise = putStrLn "2 + 3 /= 5" >> exitFailure
