# DCK version

This directory contains the construction-kit version of tcb-multi-plane-3d-scroller. The original Go sources are preserved at their original paths (revision `2fa1b3f6636c7aaa61d44ffa96f0d4366096fa21`), with small asset accessors so both versions use the same embedded resources.

Run the original with `go run ./cmd/tcb-scroller` and this version with `go run ./dck/cmd/tcb-scroller` from the repository root.

The choreography and assets stay local; reusable rendering and effects live in `../../lib/democonstructionkit`. Second Reality retains its original ST3 music synchronization.
