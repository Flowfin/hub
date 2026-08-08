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

`site-fetches-nothing-outside`, a named leg of the gate, in `internal/site`. It
refuses a served page that would load anything from another host, and a served
page that carries no content policy or one that is not restrictive by default.
Every refusal has a fixture that trips it.

The browser half is weaker than the tree half and the difference is worth
knowing. The pages are served by GitHub Pages, which serves static files and sets
no response header, so the policy is a `meta` element. A policy delivered that way
cannot carry `frame-ancestors`, `report-uri` or `sandbox`; browsers ignore those
there. The directives that stop a load from another host do work in a `meta`
element, which is what this rule needs, but a reader should not take the page as
carrying a full policy.

What the leg does not reach: everything above this section that is not a page
under `docs/`. Analytics added somewhere other than the site, a log of who fetched
the manifest, and an identifier that differs per install are all still prose. So
is a hyperlink: an anchor to another site is a navigation the visitor chooses,
nothing is sent until they click, and refusing one would be a rule against linking
rather than against tracking.
