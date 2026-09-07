# Built by builders, for builders

Agent Commons is licensed under the [Mozilla Public License 2.0](LICENSE).
This covers our source code, scripts, plugin configuration, skills and project
documentation, except material carrying its own license notice. Third-party
software keeps its own terms. Registering a project does not change its license.

This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at https://mozilla.org/MPL/2.0/.

## What that means

Use it, change it, and build something useful. Commercial use is welcome.
When you distribute binaries, make the covered source available to recipients
under MPL and tell them where to get it, even if you haven't changed the code.
Keep the required notices. Changes to covered files stay under MPL.
Separate files containing none of
our code can use another license, including a proprietary one.

Private changes can stay private. Running a modified server without distributing
its code does not, by itself, require you to share that server's source. Sharing
changes upstream is welcome, but sending us a pull request is not a license condition.

This is a short explanation, not additional terms or legal advice. The
[license](LICENSE) controls; [Mozilla's FAQ](https://www.mozilla.org/en-US/MPL/2.0/FAQ/)
explains the details, including distribution and file-level coverage.

## What we promise

Test it before trusting it with work you care about. We don't promise it will
work for your setup or provide ongoing support. Back up your work, protect your
credentials, and review what your agents do.

The software comes as is. Sections 6 and 7 of the license disclaim warranties
and limit liability, subject to applicable law. This is not a promise that every
kind of liability can be excluded everywhere.

## Source and notices

Each native archive includes `source.tar.gz` with the project source used for
the build, plus Go's license and patent notice. Unpack the source archive and run
`make build` with Go 1.26 or later. The plugin archive contains its editable source
and its own copy of the license.

The development repository is https://github.com/iksnae/agent-commons.
Public visibility and release publication are separate from this license choice;
the bundled source does not depend on access to that repository.
