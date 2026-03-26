interface Props {
  verdict: string;
}

export function VerdictBadge({ verdict }: Props) {
  if (!verdict) return null;
  const cls = `badge badge-${verdict.toLowerCase()}`;
  return <span className={cls}>{verdict}</span>;
}
