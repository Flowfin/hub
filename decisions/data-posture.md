# Nothing about a user leaves the operator's host

This repository publishes two things a stranger's machine talks to. A manifest a
Jellyfin server fetches, and a web page a person opens. Neither needs to know
anything about the people on the other end.

The posture is that personal data stays on the operator's own host unless the
operator deliberately sends it somewhere, and that this repository is not a place
where that could start happening by accident. The reason it is written before
there is anything to break is that the failure here is not a defect anyone
notices. It is a feature that lands, works, and quietly gives every operator
running these plugins a disclosure obligation they never agreed to take on.

## What this rules out

Analytics of any kind, on the site or anywhere else this project publishes. That
includes the self-hosted kind, because the visitor cannot tell the difference and
the obligation does not depend on who runs the collector.

Fonts, scripts, stylesheets, icon sets and embedded media loaded from a third
party. Each of those sends the visitor's address, their user agent and the time
of the request to somebody else the moment the page opens, before the visitor has
done anything at all. The design system already refuses a bundled typeface for an
unrelated reason, that the platform's own font is what makes an application look
native and an embedded one costs load time, and the two arguments point the same
way.

Any log of who fetched the manifest kept beyond what the hosting provider keeps
on its own account. This project does not add a layer whose purpose is to see the
requests.

An identifier per install carried in the manifest, or any value that differs
between operators. This is the shape the thing takes when somebody wants download
counts, and it turns a static file into a way of counting installations.

A form, a comment box or anything else on the site that accepts input. There is
nothing here for a visitor to submit, so there is nothing to store.

## What it does not rule out

The hosting provider sees the requests, because serving a file requires
receiving one. That is inherent and it is not concealed by anything above. What
the posture controls is what this project adds on top.

A server fetching the manifest reveals its own address to the host. Nothing here
can change that either, and the counterpart is that the manifest carries nothing
that identifies which server asked.

## Where this has to be repeated

The rule is only useful where somebody reads it before making a decision, and the
place that happens is the documents an operator opens rather than a file under
`decisions/`. These three carry it:

    README.md
    SECURITY.md
    the operator quickstart, which is #55

Writing them is milestone 8, specifically #49, and it is not done here.

## What refuses a violation

Nothing, today. Everything above is prose, and prose does not refuse a webfont.

#50 is the check: a named leg of the gate that reds when a tracked page
references an outside host, demonstrated biting on a planted reference, plus the
same rule expressed as a policy the browser enforces for visitors. Until it
lands, the posture holds because nobody has broken it yet, which is a different
statement from the posture being enforced, and it should not be written up as
the second one.
