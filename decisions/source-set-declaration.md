# The set of source repositories is declared as data in this repository

The generator reads releases from a set of repositories. That set is written down
here, as data, one record per plugin, and the generator reads nothing else to
learn what the set is.

## What was refused, and why

A list compiled into the generator. Adding a plugin becomes a code change, and
somebody reading this repository cannot see which plugins the catalogue claims
without reading source. The list is also then invisible to review as a list: a
diff shows a changed source file, not a changed catalogue.

A set derived from a query against the API, for example every non-archived
repository in an account whose name begins with a prefix. This is the one that
looks tidiest and it is the one that fails quietly. The size of the set then
changes when somebody archives a repository, renames one, makes one private, or
creates one with a matching name for an unrelated reason. None of those is a
statement about the catalogue, and all of them change it. A catalogue that
changes because of a repository setting nobody connected to the catalogue is the
same class of failure as an address that moves: no error anywhere, and the
symptom is an entry that stopped being there.

The API is still worth pointing at the world. What it is good for is checking a
declaration, which is a different job: it is allowed to be wrong out loud, and it
does not decide what the set is. That check is #24.

## What a declaration carries

Each record declares one plugin. The fields and what each one means:

`account` is the account or organisation that owns the source repository. It is
separate from the publishing organisation, and separate from whatever name the
manifest's owner field ends up carrying, because those are three facts and not
one. `decisions/names-are-data.md` is why neither name is written anywhere else.

`repository` is the repository name under that account. Account and repository are
two fields rather than one `owner/name` string, so that a rename of either is one
edit to one field rather than a string somebody has to parse to know what changed.

`slug` is the short, stable name this project uses for the plugin in its own
output: run logs, skip reports, and anywhere a plugin has to be named without
naming a repository. It exists so that a repository rename does not move the name
the project has been using for the thing.

`stable_tags` is the pattern a release's tag has to match to enter the finished
list. It is per plugin because tagging conventions are per project, and it is
declared rather than derived because the conventions in use here defeat the
obvious derivation. `decisions/channel-model.md` is where that is measured and
where the pattern for the one repository with releases today is written out.

`enabled` says whether the generator reads this record on this run. A plugin that
is temporarily not to be published is a record with this field turned off, not a
record deleted, because deleting it loses why it went away and invites somebody to
add it back next week.

`note` is free prose for the reason a field is the way it is. It is never read by
the generator. It exists because the field that gets set wrong is the one whose
reason lived in somebody's memory.

Nothing that a release already supplies is declared here. Not the version, not the
download URL, not the checksum, not the timestamp, not the plugin's identity. The
whole argument of `decisions/manifest-is-generated.md` is that a copy of a value
that lives in the release is wrong from the moment the release moves, and a
declaration file is not exempt from it. Where the plugin's identity comes from
instead is `decisions/manifest-schema.md` for what each field is and
`decisions/plugin-identity.md` for which bytes it is read out of.

## Where the declarations live

`sources/` at the root of this repository, one file per plugin, named for the
plugin's `slug`. One file rather than one list, so that adding a plugin is a new
file and a diff nobody has to read around, and so that two changes to two
different plugins do not collide in the same file.

The format is the one the generator's own toolchain reads without a dependency.
`decisions/means.md` puts that at Go and its standard library, so the declaration
files are JSON. What fixes the exact shape is the loader in #24, and the loader is
the authority for it rather than this sentence.

## The case this format has to make expressible

A declared repository with nothing to publish is the ordinary case here, not the
edge. Today it is ten of twelve:

    for r in discover invites metadata-sync requests server-pairing share-links \
             smart-collections sso stats watch-sync watchlist whisper-subtitles; do
      printf "%s %s\n" "$r" "$(gh api "repos/Flowfin/jellyfin-plugin-$r/releases?per_page=100" --jq 'length')"
    done

Run 2026-08-08. It printed 54 for `sso`, 1 for `requests` and 0 for the other ten.

So a record has to be able to say "this plugin is declared and has published
nothing" without that being indistinguishable from an error, and without anybody
having to remember which repositories were expected to be empty. The declaration
provides the expectation; what the generator does when it meets one is
`decisions/failure-posture.md`, and how it reports it is #28.

That is also why `enabled` is a field and not a deleted file. An empty repository
stays declared and enabled, because the day it publishes its first release the
catalogue should grow with no change here at all, and that property is what the
first release is proving.

## What refuses a violation

Nothing yet. This file states where the set comes from; #24 is the loader that
reads it and refuses a declaration whose repository or release it cannot resolve,
and #30 is the check that refuses either name being written into the source
instead. Until those land, the rule holds because nothing has broken it, which is
a different sentence from the rule being enforced.
