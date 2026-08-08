# The failure posture: what stops a run, what is skipped loudly, and what zero means

The generator reads a release history it does not own and cannot repair. A
published release is immutable, so a release from two years ago that is missing an
asset, or that carries a descriptor nothing can parse, will be missing it forever.
No amount of care here fixes it.

That rules out both simple answers. If any defect stops the run, one broken old
release freezes the manifest permanently and the outage is far larger than the
entry that caused it. If nothing stops the run, a build published today goes out
with a wrong value and the first person to find out is an operator whose install
fails.

## Why the line is where it is

One sentence: a defect in what this run is trying to publish is the project's own
mistake and is worth stopping for, while a defect in a release nobody can change is
a permanent fact about the world, and stopping for it every day converts somebody
else's old mistake into this project's standing outage.

## What is fatal

The run produces no output and exits non-zero. Nothing is published, and the
address keeps serving the file from the previous successful run, which is the
correct behaviour when this run cannot be trusted.

A declaration file that does not parse, or that carries a field the loader does not
accept. The declared set is this repository's own statement about what the
catalogue is, and a run that guessed past a broken statement would publish a
catalogue nobody declared.

A declared repository that does not resolve. Not the same as one with no releases.
A repository that answers with a not-found is a declaration pointing at nothing,
and continuing would silently shrink the catalogue by one plugin.

A credential or transport failure. If the run cannot read the releases it is
supposed to read, every conclusion it would draw about what exists is unfounded.
This is the case that most needs to be fatal, because its symptom is an empty or
short list, which looks exactly like success.

Any defect in a release that this run classifies as the newest one for its plugin
and channel. That is the release the run exists to publish. A missing archive, an
unpairable checksum, a descriptor that will not parse, a version string the server
cannot read: each of these in the newest release is a build that was published
wrong today, and it is worth stopping the world for while somebody is still awake
to fix it.

A version cap, an ordering rule or a byte-format rule that cannot be applied. These
are this project's own rules and a run that cannot apply one has a bug rather than
an input problem.

## What is a loud skip

The run continues, publishes what it could resolve, and names every skipped release
individually in its output, with the plugin, the tag, and which condition it hit.
The exit code stays zero. How the report is shaped is #28.

A release that is not the newest for its plugin and channel and that has any of the
defects listed as fatal above. This is the whole point of the asymmetry: an old
release that cannot be published is skipped, named, and does not stop anything.

A release whose archive predicate matches nothing. It shipped nothing this
generator recognises, which is different from a repository with no releases at all,
and the output says which.

A release whose archive predicate matches more than once, and a release where no
sidecar names the selected archive, and a release where two sidecars name it and
disagree. All four cases and their reasons are
`decisions/artifact-checksum-pairing.md`, and this file only places them: on an
older release they are skips, on the newest one they are fatal.

A release trimmed by the per-target cap. Named as trimmed rather than as defective,
because a run that reports it the same way as a broken release teaches everybody to
ignore both. `decisions/version-cap.md` is the rule.

A declared repository with no releases at all. Reported by name, counted, and not
an error. This is the ordinary case here rather than an edge:

    for r in discover invites metadata-sync requests server-pairing share-links \
             smart-collections sso stats watch-sync watchlist whisper-subtitles; do
      printf "%s %s\n" "$r" "$(gh api "repos/Flowfin/jellyfin-plugin-$r/releases?per_page=100" --jq 'length')"
    done

Run 2026-08-08. It printed 54 for `sso`, 1 for `requests` and 0 for the other ten,
so ten of twelve declared plugins are in this state today.

A declared repository whose releases all fall on the other side of the channel
split. Also reported by name and counted, and not an error, because a plugin that
has only ever published test builds has nothing to put in the finished list and
that is a true statement about it rather than a fault.

## What is never done

A version entry is never published with a field the generator could not resolve.
Not an empty checksum, not a guessed timestamp, not a version taken from the tag
because the descriptor would not parse. An entry that cannot be installed is worse
than an entry that is not offered, because the server reports the first one as a
failed install and the second one not at all.

A skip is never silent. A manifest that is short because releases were skipped and
a manifest that is short because there was nothing to add are the same file, and
only the run output can tell them apart.

## What a run that resolves zero plugins does

It is fatal. The run exits non-zero, publishes nothing, and says that it resolved
zero plugins and why each declared plugin contributed nothing.

This is the case the section exists for, because zero is where a total failure and
a true answer look identical. A run that resolved nothing because its credentials
expired, because the network was gone, or because the declaration file was empty,
produces exactly the file that a correct run over twelve empty repositories would
produce. Publishing it would replace a working catalogue with an empty one and
report success.

The counterpart is that this is a real state today and not a hypothetical, so the
posture has to survive it rather than treat it as an alarm. Eleven of the twelve
declared repositories have no releases and the twelfth has fifty-two. A run today
resolves one plugin, which is not zero, and publishes a catalogue with one entry.
`decisions/first-release.md` is why one entry is the right first release rather
than something to wait out.

The day the twelfth repository stops publishing is the day this rule earns its
keep, and it is also the day somebody will want to turn it off to get a green run.
The answer then is that an empty catalogue is a decision a person makes, by
disabling every declaration deliberately, not something a run infers from having
found nothing.

## What refuses a violation

Nothing yet. This file is prose and prose does not stop a run. The conditions above
become refusals in #24, #25, #27 and #28, and the verdict shape they share is #18.
Until those land, the posture is a rule the generator does not exist to break.
