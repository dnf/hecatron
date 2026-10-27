# The README's demo

`hct-test.gif` is `hct test` on [sayable](https://github.com/dnf/sayable): 51
languages, each tested across four libraries, plus sayable's own checks, run
in parallel with hecatron's live display. It was recorded in a container
built from the Dockerfile hecatron generates for sayable, and the 272-second
run plays at five times speed.

To remake it:

```bash
hct dockerfile                                   # in sayable, with this hecatron
docker build -f age/Dockerfile -t sayable-all .  # every toolchain sayable uses
# in a container from that image, with asciinema and agg installed:
asciinema rec --headless --window-size 110x62 --idle-time-limit 1 -c 'hct test' hct-test.cast
agg --speed 5 --fps-cap 8 --idle-time-limit 1 --last-frame-duration 4 \
    --font-family "JetBrains Mono" --font-size 14 --theme github-dark hct-test.cast hct-test.gif
```
