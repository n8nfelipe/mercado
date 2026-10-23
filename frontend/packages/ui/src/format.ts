export function formatPrice(cents: number, currency: string): string {
  const value = cents / 100;
  try {
    return new Intl.NumberFormat("pt-BR", { style: "currency", currency }).format(value);
  } catch {
    return `${currency} ${value.toFixed(2)}`;
  }
}

export function conditionLabel(condition: string): string {
  return condition === "new" ? "Novo" : "Usado";
}
