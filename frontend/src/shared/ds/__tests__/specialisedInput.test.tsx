// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Command and OtpField. #443 PR 5.
 *
 * Both are keyboard-first controls, so the keyboard contract is what is tested,
 * not the markup. Two things here are regression guards for defects that existed
 * in the hand-rolled versions these replace rather than hypotheses:
 *
 *   - a pasted one-time code that carries a space or a label ("123 456", "Code:
 *     123456") was silently rejected by all three call sites;
 *   - accented labels could not be reached from an unaccented query, which is
 *     the normal way to type on a French keyboard where the accent is a dead key.
 */

import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";

import { Command, type CommandItem } from "../Command";
import { OtpField } from "../OtpField";
import { Field } from "../Field";

/* ---------------------------------------------------------------- Command -- */

function makeItems(onSelect = vi.fn()): CommandItem[] {
  return [
    {
      id: "new-risk",
      label: "New risk",
      onSelect,
      group: "Actions",
      shortcut: "⌘N",
    },
    {
      id: "reglement",
      label: "Règlement intérieur",
      onSelect,
      group: "Navigation",
    },
    { id: "controls", label: "Contrôles", onSelect, group: "Navigation" },
    {
      id: "export",
      label: "Export register",
      onSelect,
      group: "Actions",
      keywords: ["csv"],
    },
    {
      id: "locked",
      label: "Archived tenant",
      onSelect,
      group: "Actions",
      disabled: true,
    },
  ];
}

describe("Command", () => {
  it("keeps focus in the input while arrowing, and points with aria-activedescendant", async () => {
    const user = userEvent.setup();
    render(<Command items={makeItems()} label="Commands" />);

    const input = screen.getByRole("combobox", { name: "Commands" });
    await user.click(input);
    expect(input).toHaveFocus();

    const first = input.getAttribute("aria-activedescendant");
    expect(first).toBeTruthy();

    await user.keyboard("{ArrowDown}");

    // THE CONTRACT. Menu moves focus; a combobox must not, or the user cannot
    // keep typing to narrow the list.
    expect(input).toHaveFocus();
    expect(input.getAttribute("aria-activedescendant")).not.toBe(first);
  });

  it("finds an accented label from an unaccented query", async () => {
    const user = userEvent.setup();
    render(<Command items={makeItems()} label="Commands" />);

    await user.type(screen.getByRole("combobox"), "reglement");

    // "Règlement intérieur" — reachable by someone who did not type the accent.
    expect(
      screen.getByRole("option", { name: /Règlement intérieur/ }),
    ).toBeInTheDocument();
    expect(screen.getAllByTestId("command-item")).toHaveLength(1);
  });

  it("matches keywords as well as labels", async () => {
    const user = userEvent.setup();
    render(<Command items={makeItems()} label="Commands" />);

    await user.type(screen.getByRole("combobox"), "csv");

    expect(
      screen.getByRole("option", { name: /Export register/ }),
    ).toBeInTheDocument();
  });

  it("activates the highlighted item on Enter", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<Command items={makeItems(onSelect)} label="Commands" />);

    await user.type(screen.getByRole("combobox"), "controles");
    await user.keyboard("{Enter}");

    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it("resets the highlight to the top when the query changes", async () => {
    const user = userEvent.setup();
    render(<Command items={makeItems()} label="Commands" />);
    const input = screen.getByRole("combobox");
    await user.click(input);

    await user.keyboard("{ArrowDown}{ArrowDown}");
    const moved = input.getAttribute("aria-activedescendant");

    await user.type(input, "e");

    // Leaving the highlight where it was points it at whatever happens to be in
    // that slot after filtering — which is not what the user just asked for.
    expect(input.getAttribute("aria-activedescendant")).not.toBe(moved);
    const options = screen.getAllByRole("option");
    expect(options[0]).toHaveAttribute("aria-selected", "true");
  });

  it("never activates a disabled item", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<Command items={makeItems(onSelect)} label="Commands" />);

    await user.type(screen.getByRole("combobox"), "Archived");
    await user.keyboard("{Enter}");

    expect(onSelect).not.toHaveBeenCalled();
  });

  it("shows the empty message rather than an empty box", async () => {
    const user = userEvent.setup();
    render(
      <Command
        items={makeItems()}
        label="Commands"
        emptyMessage="Nothing found"
      />,
    );

    await user.type(screen.getByRole("combobox"), "zzzz");

    expect(screen.getByText("Nothing found")).toBeInTheDocument();
    expect(screen.queryAllByRole("option")).toHaveLength(0);
  });

  it("wraps at both ends of the list", async () => {
    const user = userEvent.setup();
    render(<Command items={makeItems()} label="Commands" />);
    const input = screen.getByRole("combobox");
    await user.click(input);

    const top = input.getAttribute("aria-activedescendant");
    await user.keyboard("{ArrowUp}");
    const last = input.getAttribute("aria-activedescendant");
    expect(last).not.toBe(top);

    await user.keyboard("{ArrowDown}");
    expect(input.getAttribute("aria-activedescendant")).toBe(top);
  });
});

/* --------------------------------------------------------------- OtpField -- */

function OtpHarness({
  onComplete,
  length = 6,
  alphabet,
}: {
  onComplete?: (v: string) => void;
  length?: number;
  alphabet?: "numeric" | "alphanumeric";
}) {
  const [value, setValue] = useState("");
  return (
    <OtpField
      label="Verification code"
      value={value}
      onValueChange={setValue}
      onComplete={onComplete}
      length={length}
      alphabet={alphabet}
    />
  );
}

describe("OtpField", () => {
  it("is ONE input, not one per character", () => {
    render(<OtpHarness />);

    // The reason: paste, autocomplete="one-time-code" and screen readers all
    // address a single field. Six inputs break all three.
    expect(screen.getAllByRole("textbox")).toHaveLength(1);
    expect(screen.getAllByTestId("otp-segment")).toHaveLength(6);
  });

  it("carries the attributes that make OS autofill work", () => {
    render(<OtpHarness />);
    const input = screen.getByRole("textbox", { name: "Verification code" });

    expect(input).toHaveAttribute("autocomplete", "one-time-code");
    expect(input).toHaveAttribute("inputmode", "numeric");
    expect(input).toHaveAttribute("maxlength", "6");
  });

  it("accepts a pasted code that carries spaces or surrounding words", async () => {
    const onComplete = vi.fn();
    const user = userEvent.setup();
    render(<OtpHarness onComplete={onComplete} />);

    const input = screen.getByRole("textbox");
    await user.click(input);
    await user.paste("Your code is 123 456");

    // THE REGRESSION GUARD. Every hand-rolled version rejected this outright,
    // and the user could see the digits they had just copied.
    expect(input).toHaveValue("123456");
    expect(onComplete).toHaveBeenCalledWith("123456");
  });

  it("ignores non-digits as they are typed", async () => {
    const user = userEvent.setup();
    render(<OtpHarness />);

    const input = screen.getByRole("textbox");
    await user.type(input, "1a2b3c");

    expect(input).toHaveValue("123");
  });

  it("fires onComplete once, when the last character lands", async () => {
    const onComplete = vi.fn();
    const user = userEvent.setup();
    render(<OtpHarness onComplete={onComplete} length={4} />);

    const input = screen.getByRole("textbox");
    await user.type(input, "123");
    expect(onComplete).not.toHaveBeenCalled();

    await user.type(input, "4");
    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(onComplete).toHaveBeenCalledWith("1234");
  });

  it("does not exceed its length", async () => {
    const user = userEvent.setup();
    render(<OtpHarness length={4} />);

    const input = screen.getByRole("textbox");
    await user.click(input);
    await user.paste("123456789");

    expect(input).toHaveValue("1234");
    expect(screen.getAllByTestId("otp-segment")).toHaveLength(4);
  });

  it("upper-cases an alphanumeric backup code and takes letters", async () => {
    const user = userEvent.setup();
    render(<OtpHarness alphabet="alphanumeric" length={4} />);

    const input = screen.getByRole("textbox");
    await user.type(input, "ab3d");

    expect(input).toHaveValue("AB3D");
    expect(input).toHaveAttribute("inputmode", "text");
  });

  it("renders the typed characters in the segments", async () => {
    const user = userEvent.setup();
    render(<OtpHarness />);

    await user.type(screen.getByRole("textbox"), "42");

    const segments = screen.getAllByTestId("otp-segment");
    expect(segments[0]).toHaveTextContent("4");
    expect(segments[1]).toHaveTextContent("2");
    expect(segments[2]).toHaveTextContent("");
  });

  it("normalises a dirty value it is GIVEN, not only one that is typed", () => {
    // A caller hydrating from a store or a URL can hand over "123 456". Without
    // sanitising the incoming prop the space takes a segment of its own: the code
    // renders with a hole and the caret sits one box past where typing lands.
    render(
      <OtpField label="Code" value="123 456" onValueChange={() => {}} />,
    );

    const segments = screen.getAllByTestId("otp-segment");
    expect(segments.map((s) => s.textContent)).toEqual([
      "1",
      "2",
      "3",
      "4",
      "5",
      "6",
    ]);
    expect(screen.getByRole("textbox")).toHaveValue("123456");
  });

  it("inherits its wiring from a surrounding Field", () => {
    render(
      <Field label="Code" status="invalid" message="That code has expired">
        <OtpHarness />
      </Field>,
    );

    const input = screen.getByRole("textbox");
    expect(input).toHaveAttribute("aria-invalid", "true");
    // The error text is associated, not merely adjacent — the failure the
    // shared wiring exists to prevent.
    expect(input.getAttribute("aria-describedby")).toBeTruthy();
  });
});
