interface Props {
  value: number;
  goodDirection: 'up' | 'down';
}

export function TrendIndicator({ value, goodDirection }: Props) {
  if (value === 0) {
    return <span className="trend-neutral">0</span>;
  }

  const isPositive = value > 0;
  const isGood = goodDirection === 'up' ? isPositive : !isPositive;
  const arrow = isPositive ? '▲' : '▼';
  const colorClass = isGood ? 'trend-good' : 'trend-bad';
  const label = isPositive ? `${arrow} +${value}` : `${arrow} ${value}`;

  return <span className={colorClass}>{label}</span>;
}
