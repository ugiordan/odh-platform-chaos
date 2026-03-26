import { Alert } from '@patternfly/react-core';

interface Props {
  variant: 'info' | 'warning' | 'error';
  message: string;
}

const PF_VARIANT: Record<string, 'info' | 'warning' | 'danger'> = {
  info: 'info',
  warning: 'warning',
  error: 'danger',
};

export function StatusBanner({ variant, message }: Props) {
  if (!message) return null;
  return (
    <Alert variant={PF_VARIANT[variant]} isInline isPlain title={message} />
  );
}
