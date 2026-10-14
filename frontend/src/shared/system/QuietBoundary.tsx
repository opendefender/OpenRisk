// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Error boundary for optional pieces of the shell — the product tour, the
// shortcuts sheet, the notifications panel. Each is a lazy chunk (#966), and a
// chunk can fail to load: a flaky link, or a tab left open across a deploy. The
// root boundary would replace the whole app with the error page for something
// the user may never have asked for. This one reports the error and renders
// nothing, so the rest of the app keeps working.

import { Component, type ErrorInfo, type ReactNode } from 'react';
import { reportError } from '../../lib/observability';

interface Props {
  children: ReactNode;
}
interface State {
  failed: boolean;
}

export class QuietBoundary extends Component<Props, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    reportError(error, { componentStack: info.componentStack });
  }

  render() {
    return this.state.failed ? null : this.props.children;
  }
}
