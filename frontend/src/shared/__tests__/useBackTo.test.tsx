import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { useState } from 'react';
import { useEscapeToClose } from '../useBackTo';

function Overlay({ label, onClosed }: { label: string; onClosed: () => void }) {
  useEscapeToClose(true, onClosed);
  return <div>{label}</div>;
}

function Stack() {
  const [outer, setOuter] = useState(true);
  const [inner, setInner] = useState(true);
  return (
    <>
      {outer && <Overlay label="outer" onClosed={() => setOuter(false)} />}
      {inner && <Overlay label="inner" onClosed={() => setInner(false)} />}
    </>
  );
}

describe('useEscapeToClose', () => {
  it('closes the overlay on Escape', () => {
    let closed = false;
    render(<Overlay label="solo" onClosed={() => { closed = true; }} />);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(closed).toBe(true);
  });

  // One press must not collapse a whole stack: closing a dialog AND the drawer
  // underneath it reads as the app losing your place.
  it('closes only the topmost overlay per press', () => {
    render(<Stack />);
    expect(screen.getByText('inner')).toBeTruthy();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByText('inner')).toBeNull();
    expect(screen.getByText('outer')).toBeTruthy();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByText('outer')).toBeNull();
  });

  it('ignores other keys', () => {
    let closed = false;
    render(<Overlay label="solo" onClosed={() => { closed = true; }} />);
    fireEvent.keyDown(document, { key: 'Enter' });
    expect(closed).toBe(false);
  });
});
