import assert from 'node:assert/strict'
import { Builder, By, until } from 'selenium-webdriver'
import chrome from 'selenium-webdriver/chrome.js'

const baseUrl = process.env.REPOMANAGER_BASE_URL || 'http://127.0.0.1:8080'

describe('RepoManager web UI', function () {
  this.timeout(60000)

  let driver

  before(async () => {
    const options = new chrome.Options()
    options.addArguments('--headless=new', '--no-sandbox', '--disable-gpu')
    driver = await new Builder().forBrowser('chrome').setChromeOptions(options).build()
  })

  after(async () => {
    if (driver) await driver.quit()
  })

  it('loads the projects page', async () => {
    await driver.get(baseUrl)
    await driver.wait(until.elementLocated(By.css('body')), 10000)
    const title = await driver.getTitle()
    assert.ok(title.length > 0)
  })
})
