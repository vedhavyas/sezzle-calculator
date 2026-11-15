export type Operation = 'add' | 'subtract' | 'multiply' | 'divide' | 'power' | 'sqrt' | 'percent'

const SIGNS: Record<'add' | 'subtract' | 'multiply' | 'divide' | 'power', string> = {
  add: '+',
  subtract: '−',
  multiply: '×',
  divide: '÷',
  power: '^',
}

export function expressionFor(operation: Operation, left: string, right?: string): string {
  if (operation === 'sqrt') return `√${left}`
  if (operation === 'percent') return `${left}% of ${right}`
  return `${left} ${SIGNS[operation]} ${right}`
}

export function pendingExpression(operation: Operation, left: string): string {
  if (operation === 'sqrt') return `√${left}`
  if (operation === 'percent') return `${left}% of`
  return `${left} ${SIGNS[operation]}`
}
