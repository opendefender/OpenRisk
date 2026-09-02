// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #474 — the one-time-code fields, after moving onto shared/ds.
//
// Two different controls, and the difference is the whole point of this suite:
//
//   ENROLMENT   only ever takes the 6-digit TOTP code the authenticator shows.
//               Fixed length, so it is an OtpField and is drawn as segments.
//   LOGIN       takes a 6-digit TOTP code OR a 12-character recovery code
//               (backend MFAVerifyInput.Code is documented "TOTP code OR backup
//               code", and pkg/otp/totp.go mints 12-char base32 recovery codes).
//               NOT fixed length, so NOT an OtpField — segmenting it would cap
//               entry at six characters and lock out exactly the users whose
//               authenticator is gone.
//
// Both share the defect this issue exists to fix: a code pasted with spaces, or
// with the words around it, was silently rejected while the user could plainly
// read the digits on screen.

import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";

const navigate = vi.fn();
const login = vi.fn();
const adoptSession = vi.fn();
const post = vi.fn();
const setupMFA = vi.fn();
const verifyMFA = vi.fn();
const challengeMFA = vi.fn();

vi.mock("react-router", async () => {
  const actual =
    await vi.importActual<typeof import("react-router")>("react-router");
  return { ...actual, useNavigate: () => navigate };
});
vi.mock("../../../lib/api", () => ({
  api: { post: (...a: unknown[]) => post(...a), defaults: { baseURL: "" } },
}));
vi.mock("../authService", () => ({
  setupMFA: (...a: unknown[]) => setupMFA(...a),
  verifyMFA: (...a: unknown[]) => verifyMFA(...a),
  challengeMFA: (...a: unknown[]) => challengeMFA(...a),
  listSessions: vi.fn(),
  revokeSession: vi.fn(),
  revokeOtherSessions: vi.fn(),
}));
vi.mock("../../../hooks/useAuthStore", () => {
  const state = {
    user: undefined,
    login: (...a: unknown[]) => login(...a),
    adoptSession: (...a: unknown[]) => adoptSession(...a),
  };
  const useAuthStore = (sel?: (s: typeof state) => unknown) =>
    sel ? sel(state) : state;
  useAuthStore.getState = () => state;
  return { useAuthStore };
});

import { AuthScreen } from "../AuthScreen";

function renderAt(initialView: "login" | "register") {
  return render(
    <MemoryRouter>
      <AuthScreen initialView={initialView} />
    </MemoryRouter>,
  );
}

/** Drives a login all the way to the MFA challenge field. */
async function reachLoginChallenge() {
  const user = userEvent.setup();
  login.mockResolvedValue({
    status: "mfa_required",
    mfa_token: "challenge-token",
  });
  renderAt("login");
  await user.type(screen.getByTestId("login-email"), "alix@example.com");
  await user.type(screen.getByTestId("login-password"), "MotDePasse2026!");
  await user.click(screen.getByTestId("login-submit"));
  return { user, input: await screen.findByTestId("mfa-code") };
}

/** Drives a registration all the way to the MFA enrolment field. */
async function reachEnrolment() {
  const user = userEvent.setup();
  login.mockResolvedValue({
    status: "mfa_enrollment_required",
    mfa_token: "enrol-token",
  });
  renderAt("register");
  await user.type(screen.getByTestId("register-name"), "Alix Mensah");
  await user.type(screen.getByTestId("register-email"), "alix@example.com");
  await user.type(screen.getByTestId("register-password"), "MotDePasse2026!");
  await user.click(screen.getByTestId("register-submit"));
  await waitFor(() => expect(setupMFA).toHaveBeenCalled());
  return { user, input: await screen.findByTestId("mfa-enrol-code") };
}

beforeEach(() => {
  vi.clearAllMocks();
  post.mockResolvedValue({ data: {} });
  setupMFA.mockResolvedValue({
    secret: "ABCDEF",
    qr_code: "/9j/rawbase64",
    backup_codes: ["ABCDEFGH2345"],
  });
});

/* ------------------------------------------------------------------ login -- */

describe("the MFA login field", () => {
  it("accepts a code pasted with spaces", async () => {
    const { user, input } = await reachLoginChallenge();

    await user.click(input);
    await user.paste("123 456");

    // THE DEFECT. This used to arrive as "123 456", which the backend rejects.
    expect(input).toHaveValue("123456");
  });

  it("still accepts a 12-character recovery code — it is NOT capped at six", async () => {
    const { user, input } = await reachLoginChallenge();

    await user.click(input);
    await user.paste("ABCDEFGH2345");

    // THE REGRESSION GUARD for this issue's own scope. Making this an OtpField
    // would have truncated to 6 and locked out every user who has lost their
    // authenticator — the one situation recovery codes exist for.
    expect(input).toHaveValue("ABCDEFGH2345");
  });

  it("upper-cases a recovery code typed in lower case", async () => {
    const { user, input } = await reachLoginChallenge();

    // The codes are displayed upper-case and the alphabet is upper-case base32;
    // typing them as read should work.
    await user.type(input, "abcdefgh2345");

    expect(input).toHaveValue("ABCDEFGH2345");
  });

  it("is still a single field carrying the OS autofill hints", async () => {
    const { input } = await reachLoginChallenge();

    expect(input).toHaveAttribute("autocomplete", "one-time-code");
    expect(input).toHaveAttribute("inputmode", "numeric");
  });
});

/* -------------------------------------------------------------- enrolment -- */

describe("the MFA enrolment field", () => {
  it("is an OtpField of exactly six segments", async () => {
    const { input } = await reachEnrolment();

    // It used to be a bare input with maxLength={8} — wrong for a 6-digit code
    // in one direction and for a 12-char recovery code in the other, and
    // recovery codes are never entered here in the first place.
    expect(input).toHaveAttribute("maxlength", "6");
    expect(screen.getAllByTestId("otp-segment")).toHaveLength(6);
  });

  it("accepts a code pasted with spaces", async () => {
    const { user, input } = await reachEnrolment();

    await user.click(input);
    await user.paste("123 456");

    expect(input).toHaveValue("123456");
  });

  it("drops non-digits rather than refusing the code", async () => {
    const { user, input } = await reachEnrolment();

    await user.click(input);
    await user.paste("Your code is 654321");

    expect(input).toHaveValue("654321");
  });
});
