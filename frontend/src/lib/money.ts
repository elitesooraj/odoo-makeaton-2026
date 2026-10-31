export function formatCents(cents: number, currency = 'USD'): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(cents / 100);
}

export function centsToDecimal(cents: number): string {
  return (cents / 100).toFixed(2);
}

export function dollarsToCents(dollars: string | number): number {
  if (typeof dollars === 'number') {
    return Math.round(dollars * 100);
  }
  const clean = dollars.replace(/[^0-9.-]/g, '');
  const parsed = parseFloat(clean);
  if (isNaN(parsed)) return 0;
  return Math.round(parsed * 100);
}

export function formatBasisPoints(bp: number): string {
  return (bp / 100).toFixed(2) + '%';
}
