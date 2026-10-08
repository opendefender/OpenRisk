---
name: shake-remounts-detach-node-refs
description: "shared/ds/Shake (and features/auth/fields.tsx's copy) remounts its children via React key={errorKey} on every new error nonce — a DOM node reference captured before a failed submit is detached afterward; tests must re-query rather than reuse the handle."
metadata:
  type: feedback
---

`Shake` (`frontend/src/shared/ds/Shake.tsx`, and the auth-screen-local copy in
`features/auth/fields.tsx`) shakes its children by using the caller's
`errorKey` (an incrementing nonce, bumped once per failed submit) as the
React `key` on a wrapping `<div>`. A changed `key` forces React to unmount
the old subtree and mount a fresh one — that is how the CSS shake animation
restarts on a SECOND identical-looking failure, which is the component's
whole reason to exist (see its own doc comment).

**The trap:** any `<input>` (or other element) captured via
`screen.getByRole(...)`/`getByTestId(...)` etc. BEFORE a submit that bumps
the nonce is a reference to a node that gets detached from the DOM the
moment that submit's catch branch runs. Reusing that stale handle afterward
(e.g. `userEvent.click(oldInput)`, `userEvent.clear(oldInput)`) fails with
"The element to be cleared could not be focused" or silently does nothing,
because the element is no longer attached to `document`.

**How to apply:** whenever a test drives a `Shake`-wrapped field through a
failed submit and then needs to interact with it again (retype, clear, edit
to check an error clears), re-query the live node with
`screen.getByTestId(...)` / `screen.getByRole(...)` AFTER the failure,
instead of reusing a `const input = ...` captured earlier in the test. Found
building #751 phase 5's MFA-enrolment auto-submit tests
(`mfaOtpAutoSubmit.test.tsx`), where the AuthScreen `MFAEnrollment` field is
wrapped in `Shake` and `MFAEnrollmentDialog`'s is not — the same "type after
a rejected code" interaction needed the re-query only on the AuthScreen side.
