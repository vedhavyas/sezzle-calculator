import { expect, test, type Page } from '@playwright/test'

async function press(page: Page, names: string[]) {
  for (const name of names) await page.getByRole('button', { name, exact: true }).click()
}

test('adds through the API and shows the completed expression', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByTestId('value')).toHaveText('0')

  await press(page, ['1', '2', 'add', '3', 'equals'])

  await expect(page.getByTestId('value')).toHaveText('15')
  await expect(page.getByTestId('expression')).toHaveText('12 + 3 =')
})

test('keeps a decimal addition exact', async ({ page }) => {
  await page.goto('/')
  await press(page, ['0', 'decimal point', '1', 'add', '0', 'decimal point', '2', 'equals'])

  await expect(page.getByTestId('value')).toHaveText('0.3')
})

test('shows the server refusal and recovers', async ({ page }) => {
  await page.goto('/')
  await press(page, ['1', '0', 'divide', '0', 'equals'])

  await expect(page.getByTestId('value')).toHaveText('division by zero is undefined')
  await expect(page.getByTestId('value')).toHaveAttribute('data-error', 'true')

  await page.getByRole('button', { name: 'clear', exact: true }).click()
  await expect(page.getByTestId('value')).toHaveText('0')
})

test('evaluates left to right', async ({ page }) => {
  await page.goto('/')
  await press(page, ['2', 'add', '3', 'multiply', '4', 'equals'])

  await expect(page.getByTestId('value')).toHaveText('20')
})

test('drives the calculator from the keyboard', async ({ page }) => {
  await page.goto('/')
  await page.keyboard.type('7*6')
  await page.keyboard.press('Enter')

  await expect(page.getByTestId('value')).toHaveText('42')
})

test('fits a phone screen without horizontal scroll', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 740 })
  await page.goto('/')
  await press(page, ['9', 'multiply', '9', 'equals'])

  await expect(page.getByTestId('value')).toHaveText('81')
  const overflows = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
  )
  expect(overflows).toBe(false)
})

const landscapeViewports = [
  { width: 844, height: 390, minKeyHeight: 44 },
  { width: 667, height: 375, minKeyHeight: 44 },
  { width: 640, height: 360, minKeyHeight: 38 },
  { width: 568, height: 320, minKeyHeight: 38 },
]

for (const viewport of landscapeViewports) {
  test(`fits ${viewport.width}x${viewport.height} without scrolling`, async ({ page }) => {
    await page.setViewportSize({ width: viewport.width, height: viewport.height })
    await page.goto('/')
    await press(page, ['9', 'multiply', '9', 'equals'])

    await expect(page.getByTestId('value')).toHaveText('81')
    await expect(page.getByRole('button', { name: 'equals', exact: true })).toBeInViewport()

    const metrics = await page.evaluate(() => ({
      vertical: document.documentElement.scrollHeight > document.documentElement.clientHeight,
      horizontal: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      keyHeight: (document.querySelector('[aria-label="7"]') as HTMLElement).getBoundingClientRect().height,
    }))
    expect(metrics.vertical).toBe(false)
    expect(metrics.horizontal).toBe(false)
    expect(metrics.keyHeight).toBeGreaterThanOrEqual(viewport.minKeyHeight)
  })
}

test('loads the bundled fonts rather than falling back', async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => document.fonts.ready)

  // A loaded face, not check(), which answers true for a family that does not exist.
  const loadedFamilies = await page.evaluate(() =>
    Array.from(document.fonts)
      .filter((face) => face.status === 'loaded')
      .map((face) => face.family),
  )
  expect(loadedFamilies.some((family) => family.includes('IBM Plex Mono'))).toBe(true)
  expect(loadedFamilies.some((family) => family.includes('IBM Plex Sans'))).toBe(true)

  const family = await page.getByTestId('value').evaluate((element) => getComputedStyle(element).fontFamily)
  expect(family).toContain('IBM Plex Mono')
})

test('switches to dark colours when the system asks', async ({ browser }) => {
  const light = await browser.newContext({ colorScheme: 'light' })
  const dark = await browser.newContext({ colorScheme: 'dark' })

  const lightPage = await light.newPage()
  const darkPage = await dark.newPage()
  await lightPage.goto('/')
  await darkPage.goto('/')

  const background = (page: Page) =>
    page.evaluate(() => getComputedStyle(document.body).backgroundColor)

  const lightBackground = await background(lightPage)
  const darkBackground = await background(darkPage)

  expect(darkBackground).not.toBe(lightBackground)
  await expect(darkPage.getByTestId('value')).toHaveText('0')

  await light.close()
  await dark.close()
})

test('cycles the theme and keeps it after a reload', async ({ page }) => {
  await page.goto('/')
  const toggle = page.getByRole('button', { name: /^Theme:/ })

  await expect(toggle).toHaveText('Theme: System')
  await toggle.click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await toggle.click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(13, 17, 23)')

  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.getByRole('button', { name: /^Theme:/ })).toHaveText('Theme: Dark')
})
