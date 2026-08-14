# How each number in the speed budget is measured

The design system publishes five numbers. They are the strongest thing in it,
because a promise with a number is one a build can miss, and they are worth
nothing until two people measuring the same client get the same answer.

Without a method they will not. Focus change from key event to first changed
pixel and focus change from key event to callback are different numbers, and on a
television they are wide apart. First usable content depends on what counts as
usable and on whether the cache was warm. Playback start depends on whether the
server is transcoding, which is not a property of the client at all.

So each number below carries what starts the clock, what stops it, the device and
the network, how many runs, and which statistic is reported. The percentile,
because the complaint a person has is about the worst thing they noticed.

Where a number cannot be measured on a clean runner it says so and names the
harness requirement it needs. `decisions/headless-and-unelevated.md` is why that
requirement carries a name, and `internal/harness` is where the names are. A
budget that quietly stopped being checked is worse than one that says which
parts are not.

## Focus change, under 80 ms

**Starts** at the key event as the device reports it: the down edge, not the up
edge, and taken from the platform's own input timestamp, which is stamped before
the application loop notices it. **Stops** at the first frame presented in which any
pixel of the newly focused element differs from the frame before it. Presented,
not painted, and not the callback: what the person waits for is the screen.

**Device and network.** The slowest device in the supported set, on mains power,
with the application already running and the row already scrolled to. The network
is not in this measurement and the row is warm, because a focus move that has to
fetch is a different failure.

**Runs and statistic.** 100 focus moves along one row, alternating direction, and
the reported number is the 95th percentile. The maximum is recorded beside it,
because one 300 ms move is a thing a person sees and a mean hides.

**Where it runs.** `needs-browser` for a web client and hardware for a native
one. Not measurable on a clean runner: the number is about frames presented on a
display and there is no display in the gate. The frame-timing part of it is
measurable in a headless browser with a compositor, and what that yields is an
approximation, recorded under that word.

## Dropped frames, none at 60 fps

**Starts** when a scroll of 200 tiles begins, driven by a synthetic input at a
fixed velocity, so two runs are comparable; a hand repeats nothing closely
enough to compare. **Stops** when the scroll ends and the content is at rest.

A dropped frame is a presented frame whose interval from the previous presented
frame exceeds 1.5 times the display's frame period. That threshold is stated
rather than assumed: at 60 Hz a 16.7 ms period and a 25 ms gap is one frame
missed, and a rule reading "over 16.7 ms" counts scheduling noise as a drop and
reports a number nobody can hit.

**Device and network.** The slowest supported device, artwork already in the
local cache. Fetching artwork mid-scroll is a real failure and it belongs beside
the first number: this one measures whether the client can present what it
already has.

**Runs and statistic.** 10 scrolls, and the reported number is the total count of
dropped frames across all 10, not an average. The budget is zero, so an average
of 0.3 is a fail dressed as a pass.

**Where it runs.** `needs-browser` for a web client, hardware for a native one.
Not measurable on a clean runner.

## First usable content, under 1.2 s from cold

**Usable** is defined before the clock is, because everything else here follows
from it: the first tile row is laid out at its final size, the first tile's title
is legible, and the row responds to a focus move. Artwork is not required to have
arrived. A grey tile at the right size that moves when you press a key is usable;
the same tile that is about to resize is not.

**Starts** at process launch as the platform reports it, which is earlier than
the application's own first line and is the point the person's wait starts at.
**Stops** at the first presented frame meeting the definition above.

**Cold** is stated because it is where the number is usually lost: no warm
process, no warm page cache, no pre-rendered view, and the artwork cache emptied.
The device is rebooted before each run; restarting the application leaves too
much of the machine warm.

**Device and network.** Slowest supported device. A local network with the server
on the same subnet, because a cold start makes requests and the number is about
the client rather than about somebody's uplink.

**Runs and statistic.** 10 cold starts, reported as the median and the maximum.
Both, because a cold start has a long tail and the median alone hides the run
that made somebody close the app.

**Where it runs.** Hardware, or `needs-browser` with the cache and the profile
cleared between runs. Not measurable on a clean runner, and a reboot per run is
the part that keeps it off any shared machine.

## Layout shift, none

**Starts** at the same launch point as the number above and **stops** 5 seconds
after the first usable frame, which is long enough for artwork and metadata to
arrive and short enough not to fold ordinary navigation into the measurement.

The measured quantity is every movement of a laid-out element that the person did
not ask for, summed. On the web that is the cumulative layout shift score with a
budget of 0.00 rather than the 0.1 the ecosystem calls good, because the design
system's claim is that the tile size is fixed before the image arrives, and 0.1
is the claim that it usually is. On a native client the same rule is a test that
no view's frame changes after it has been presented, except in response to input.

**Device and network.** Slowest supported device, and a throttled connection
rather than a fast one. A slow image is what makes a shift visible, so measuring
this on a fast network is measuring the wrong case.

**Runs and statistic.** 10 runs, reported as the maximum. A budget of zero has no
percentile.

**Where it runs.** `needs-browser` for a web client, hardware for a native one.
The static half of it is a gate check, because a tile whose height is not fixed
in the stylesheet is visible in the source; the dynamic half needs the harness.

## Playback start, under 2 s

**Starts** at the input that selects the item. **Stops** at the first presented
frame of video, not at the first byte, not at the player's ready event, and not
at the audio starting.

**Device and network.** Slowest supported device, wired local network, server on
the same subnet. **Direct play only**, and this is the condition that decides
whether the number means anything: a transcoded start measures the server's
processor and says nothing about the client. A run where the server transcoded is
discarded and reported as discarded rather than averaged in.

**Runs and statistic.** 20 starts across at least 3 distinct items, reported as
the 95th percentile, with the count of discarded transcodes beside it.

**Where it runs.** `needs-jellyfin`, and `needs-browser` or hardware as well. It
needs a real server holding a real file, so it is the furthest of the five from
anything a merge gate could do.

## What this document does not do

It measures nothing. Every number above is a method, and no run has been made
against any client, because there is no client in this tree yet. Reading this as
evidence that a client meets the budget would be reading it exactly backwards.

It does not add a check. Four of the five need a display and the fifth needs a
server, so the harness requirements are named and nothing here is a leg of the
gate. The static halves worth having in the gate are named where they exist,
and building them is not this document's.

It does not fix the numbers themselves. Those are the design system's and
`docs/design-system.html` is where they are published; if a number moves, it
moves there and this file follows.
