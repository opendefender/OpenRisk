// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Top-level error boundary. Catches render/lifecycle errors anywhere in the tree,
// reports them (Sentry when present), and shows the branded 500 page with a
// correlation id and a working way out — instead of React unmounting to a blank
// white screen. Resetting on a route change lets the user navigate away from a
// screen that threw.

import { Component, type ErrorInfo, type ReactNode } from 'react';
import { reportError } from '../../lib/observability';
import { ServerError } from './ServerError';

interface Props {
  children: ReactNode;
}
interface State {
  hasError: boolean;
  errorId?: string;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    const errorId = reportError(error, { componentStack: info.componentStack });
    this.setState({ errorId });
  }

  private reset = () => this.setState({ hasError: false, errorId: undefined });

  render() {
    if (this.state.hasError) {
      return <ServerError errorId={this.state.errorId} onRetry={this.reset} />;
    }
    return this.props.children;
  }
}
