type Props = {
  expression: string
  value: string
  isError: boolean
}

function sizeFor(value: string): 'short' | 'medium' | 'long' {
  if (value.length > 18) return 'long'
  if (value.length > 12) return 'medium'
  return 'short'
}

export function Display({ expression, value, isError }: Props) {
  return (
    <div className="display">
      <output className="display__expression" data-testid="expression">
        {expression}
      </output>
      <output
        className="display__value"
        data-testid="value"
        data-size={sizeFor(value)}
        data-error={isError ? 'true' : undefined}
        aria-live="polite"
        aria-atomic="true"
      >
        {value}
      </output>
    </div>
  )
}
