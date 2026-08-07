# The versions list is capped per target, and the cap is five

A manifest that lists every version ever published grows without limit, and every
server fetches the whole file whenever it looks. So the versions list is capped.

## The cap applies per target ABI, not across the whole list

A single overall cap has a quiet failure. Two target ABIs share one versions
array, and the server decides which entries it can see by comparing its own
version against each entry's `targetAbi`:

    curl -sSL -o im.cs https://raw.githubusercontent.com/jellyfin/jellyfin/master/Emby.Server.Implementations/Updates/InstallationManager.cs
    grep -n 'TargetAbi) <= appVer' im.cs
    269:                .Where(x => string.IsNullOrEmpty(x.TargetAbi) || Version.Parse(x.TargetAbi) <= appVer);

Run 2026-08-08. An entry built for a newer line is filtered out by every server
still on the older one. So when the newer line publishes far more often, an
overall cap pushes the older line's entries off the end, and from then on a
server on that line sees a plugin with no installable version at all. Nothing is
red, the manifest is well-formed, and the plugin has quietly disappeared for one
group of users. That has happened in a neighbouring project, and preventing it is
the whole reason this rule is per target.

Capping per target means a slowly moving line keeps its entries however fast the
other one moves. It costs a longer file and nothing else.

## Why five

Five is a trade between the bytes every server pulls and how far back an operator
can roll a plugin, and the byte side of it is measurable rather than assumed. The
Jellyfin project's own published catalogue gives the size of a real version
entry:

    curl -sSL -o jf-manifest.json https://repo.jellyfin.org/files/plugin/manifest.json
    python -c "import json,statistics; d=json.load(open('jf-manifest.json')); s=[len(json.dumps(v,separators=(',',':'))) for p in d for v in p['versions']]; print(len(d),'plugins',len(s),'entries median',statistics.median(s),'mean',round(statistics.mean(s)))"
    34 plugins 277 entries median 361 mean 425

Run 2026-08-08. At that mean, twelve plugins across three live ABI lines cost
roughly this at each cap:

    python -c "
    for cap in (3,5,8): print(cap, 12*3*cap, 'entries', round(12*3*cap*425), 'bytes')"
    3 108 entries 45900 bytes
    5 180 entries 76500 bytes
    8 288 entries 122400 bytes

Run 2026-08-08. Three saves 30 KB against five and takes the rollback depth down
to two builds back, which is inside the range where the release that broke
something is still the current one. Eight costs 46 KB over five, and what it buys
is depth into releases that are already old. In the same catalogue the
fifth-newest entry is a median 1462 days behind today and the eighth-newest is
1769:

    python -c "
    import json,statistics,datetime
    d=json.load(open('jf-manifest.json')); now=datetime.datetime(2026,8,8)
    def age(vs,n):
        vs=sorted(vs,key=lambda v:v['timestamp'],reverse=True)
        return None if len(vs)<n else (now-datetime.datetime.strptime(vs[n-1]['timestamp'][:10],'%Y-%m-%d')).days
    for n in (5,8):
        a=[x for x in (age(p['versions'],n) for p in d) if x is not None]
        print('nth',n,'plugins with that many:',len(a),'median age days:',int(statistics.median(a)))"
    nth 5 plugins with that many: 23 median age days: 1462
    nth 8 plugins with that many: 18 median age days: 1769

Run 2026-08-08. That measures a catalogue whose plugins publish rarely, so it is
an upper bound on how far back five reaches rather than a prediction about this
one. It is enough to say that the extra three entries buy time an operator is
unlikely to want and pay for it on every fetch. Five sits where the file is still
small enough to be uninteresting and the rollback is deep enough to cover the
case it exists for.

The number is a judgement about a trade, not a measurement, and only the byte
side of it was measured. It is a number in one place and changing it is one edit.

## What an operator loses at five

An operator can install or roll back to the newest five versions on their own ABI
line, and no further through the server interface. The sixth-newest and older are
still published on their release pages and can still be downloaded and installed
by hand, so what the cap removes is convenience, not access.

The case where that hurts is a plugin that publishes several versions in quick
succession, because five entries can then span a short period, and an operator
who wants the build from before that run has to go to the release page. The cap
does not measure time.
