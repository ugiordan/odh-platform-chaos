import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { VerdictBadge } from './VerdictBadge';
import { PhaseBadge } from './PhaseBadge';
import { StatusBanner } from './StatusBanner';
import { TrendIndicator } from './TrendIndicator';
import { PhaseStepper } from './PhaseStepper';
import { ProgressBar } from './ProgressBar';
import { CoverageMatrix } from './CoverageMatrix';

describe('VerdictBadge', () => {
  it('renders verdict text with correct class', () => {
    const { container } = render(<VerdictBadge verdict="Resilient" />);
    const badge = container.querySelector('.badge');
    expect(badge).toHaveTextContent('Resilient');
    expect(badge).toHaveClass('badge-resilient');
  });

  it('renders nothing when verdict is empty', () => {
    const { container } = render(<VerdictBadge verdict="" />);
    expect(container.firstChild).toBeNull();
  });
});

describe('PhaseBadge', () => {
  it('renders display name for CRD phase', () => {
    render(<PhaseBadge phase="SteadyStatePre" />);
    expect(screen.getByText('Pre-check')).toBeInTheDocument();
  });
});

describe('StatusBanner', () => {
  it('renders error variant', () => {
    render(<StatusBanner variant="error" message="Something broke" />);
    expect(screen.getByText('Something broke')).toBeInTheDocument();
  });

  it('renders nothing when message is empty', () => {
    const { container } = render(<StatusBanner variant="info" message="" />);
    expect(container.firstChild).toBeNull();
  });
});

describe('TrendIndicator', () => {
  it('shows positive trend with up arrow', () => {
    render(<TrendIndicator value={5} goodDirection="up" />);
    expect(screen.getByText(/\+5/)).toBeInTheDocument();
  });

  it('shows negative trend with down arrow', () => {
    render(<TrendIndicator value={-3} goodDirection="up" />);
    expect(screen.getByText(/-3/)).toBeInTheDocument();
  });

  it('shows zero as neutral', () => {
    render(<TrendIndicator value={0} goodDirection="up" />);
    expect(screen.getByText('0')).toBeInTheDocument();
  });
});

describe('PhaseStepper', () => {
  it('renders all phase steps', () => {
    render(<PhaseStepper currentPhase="Injecting" />);
    expect(screen.getByText('Pending')).toBeInTheDocument();
    expect(screen.getByText('Pre-check')).toBeInTheDocument();
    expect(screen.getByText('Injecting')).toBeInTheDocument();
    expect(screen.getByText('Observing')).toBeInTheDocument();
  });

  it('marks completed phases as done', () => {
    const { container } = render(<PhaseStepper currentPhase="Observing" />);
    const dots = container.querySelectorAll('.step-dot');
    expect(dots[0]).toHaveClass('done');
    expect(dots[1]).toHaveClass('done');
    expect(dots[2]).toHaveClass('done');
    expect(dots[3]).toHaveClass('active');
  });

  it('handles Aborted phase', () => {
    const { container } = render(<PhaseStepper currentPhase="Aborted" abortedAtPhase="Injecting" />);
    const dots = container.querySelectorAll('.step-dot');
    expect(dots[2]).toHaveClass('aborted');
  });
});

describe('ProgressBar', () => {
  it('renders segments with correct widths', () => {
    const { container } = render(<ProgressBar resilient={7} degraded={2} failed={1} />);
    const segments = container.querySelectorAll('.progress-segment');
    expect(segments).toHaveLength(3);
  });

  it('handles all zeros gracefully', () => {
    const { container } = render(<ProgressBar resilient={0} degraded={0} failed={0} />);
    const segments = container.querySelectorAll('.progress-segment');
    expect(segments).toHaveLength(0);
  });
});

describe('CoverageMatrix', () => {
  it('renders injection type columns', () => {
    render(<CoverageMatrix experiments={[]} />);
    expect(screen.getByText('PodKill')).toBeInTheDocument();
    expect(screen.getByText('ConfigDrift')).toBeInTheDocument();
  });

  it('shows tested count for injection types with experiments', () => {
    const exps = [
      { type: 'PodKill', verdict: 'Resilient' },
      { type: 'PodKill', verdict: 'Resilient' },
    ];
    render(<CoverageMatrix experiments={exps as any} />);
    expect(screen.getByText(/2x/)).toBeInTheDocument();
  });
});
