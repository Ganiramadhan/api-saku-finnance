import http from 'k6/http'
import { check, fail, group, sleep } from 'k6'
import { Rate, Trend } from 'k6/metrics'

const BASE_URL = trimTrailingSlash(__ENV.SAKU_BASE_URL || 'http://localhost:8080')
const WEB_URL = trimTrailingSlash(__ENV.SAKU_WEB_URL || BASE_URL)
const API_URL = trimTrailingSlash(__ENV.SAKU_API_URL || `${BASE_URL}/api/v1`)
const PROFILE = (__ENV.SAKU_PROFILE || 'smoke').toLowerCase()
const AUTHENTICATED = envBool('SAKU_AUTHENTICATED', PROFILE !== 'smoke')
const THINK_TIME_MIN = envNumber('SAKU_THINK_TIME_MIN', 1)
const THINK_TIME_MAX = envNumber('SAKU_THINK_TIME_MAX', 3)

const requestErrors = new Rate('saku_request_errors')
const dashboardDuration = new Trend('saku_dashboard_duration', true)

const fileTokens = loadTokenFile()
const inlineTokens = (__ENV.SAKU_TOKENS || __ENV.SAKU_TOKEN || '')
  .split(',')
  .map((token) => token.trim())
  .filter(Boolean)
const configuredTokens = fileTokens.concat(inlineTokens)

guardDangerousTargets()

export const options = buildOptions(PROFILE)

export function setup() {
  if (!AUTHENTICATED) return { tokens: [] }
  if (configuredTokens.length > 0) return { tokens: configuredTokens }

  const email = (__ENV.SAKU_EMAIL || '').trim()
  const password = __ENV.SAKU_PASSWORD || ''
  if (!email || !password) {
    fail(
      'Authenticated profile requires SAKU_TOKEN, SAKU_TOKENS, SAKU_TOKEN_FILE, or SAKU_EMAIL + SAKU_PASSWORD.',
    )
  }

  const response = http.post(
    `${API_URL}/auth/login`,
    JSON.stringify({
      email,
      password,
      turnstile_token: __ENV.SAKU_TURNSTILE_TOKEN || undefined,
    }),
    {
      headers: jsonHeaders(),
      tags: { endpoint: 'auth_login', flow: 'setup' },
      timeout: '15s',
    },
  )
  const valid = check(response, {
    'setup login returns 200': (res) => res.status === 200,
    'setup login returns token': (res) => Boolean(readToken(res)),
  })
  if (!valid) {
    fail(`Load-test login failed (${response.status}): ${truncate(response.body, 300)}`)
  }
  return { tokens: [readToken(response)] }
}

export default function (data) {
  if (AUTHENTICATED) {
    authenticatedUserJourney(selectToken(data.tokens))
  } else {
    publicUserJourney()
  }
  sleep(randomBetween(THINK_TIME_MIN, THINK_TIME_MAX))
}

function publicUserJourney() {
  group('public website', () => {
    const home = http.get(`${WEB_URL}/`, requestParams('home_page'))
    record(home, 'home page returns successfully', [200])

    const health = http.get(`${BASE_URL}/health`, requestParams('health'))
    record(health, 'health endpoint returns 200', [200])

    const plans = http.get(`${API_URL}/subscriptions/plans`, requestParams('subscription_plans'))
    record(plans, 'subscription plans return 200', [200])
  })
}

function authenticatedUserJourney(token) {
  const headers = authHeaders(token)

  group('dashboard read workload', () => {
    const startedAt = Date.now()
    const responses = http.batch([
      ['GET', `${API_URL}/users/me`, null, requestParams('users_me', headers)],
      ['GET', `${API_URL}/wallets`, null, requestParams('wallets', headers)],
      ['GET', `${API_URL}/transactions?page=1&limit=10`, null, requestParams('transactions', headers)],
      ['GET', `${API_URL}/categories`, null, requestParams('categories', headers)],
      ['GET', `${API_URL}/upcoming-billings`, null, requestParams('upcoming_billings', headers)],
      ['GET', `${API_URL}/subscriptions/me`, null, requestParams('subscriptions_me', headers)],
      ['GET', `${API_URL}/notifications?page=1&limit=10`, null, requestParams('notifications', headers)],
    ])
    dashboardDuration.add(Date.now() - startedAt)

    const labels = [
      'users/me',
      'wallets',
      'transactions',
      'categories',
      'upcoming-billings',
      'subscriptions/me',
      'notifications',
    ]
    responses.forEach((response, index) => {
      record(response, `${labels[index]} returns 200`, [200])
    })
  })

  // One additional read per iteration approximates navigation after opening the dashboard.
  const secondaryReads = [
    ['/budgets', 'budgets'],
    ['/savings-goals', 'savings_goals'],
    ['/wallets/transfers?limit=20', 'wallet_transfers'],
    ['/ai/chat-history?page=1&limit=10', 'chat_history'],
  ]
  const selected = secondaryReads[Math.floor(Math.random() * secondaryReads.length)]
  const response = http.get(`${API_URL}${selected[0]}`, requestParams(selected[1], headers))
  record(response, `${selected[1]} returns 200`, [200])
}

function record(response, checkName, acceptedStatuses) {
  const passed = check(response, {
    [checkName]: (res) => acceptedStatuses.includes(res.status),
  })
  requestErrors.add(!passed)
}

function selectToken(tokens) {
  if (!tokens || tokens.length === 0) fail('No authentication token is available.')
  return tokens[(__VU - 1) % tokens.length]
}

function requestParams(endpoint, headers = {}) {
  return {
    headers,
    tags: { endpoint },
    timeout: __ENV.SAKU_REQUEST_TIMEOUT || '15s',
  }
}

function jsonHeaders() {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  }
}

function authHeaders(token) {
  return {
    ...jsonHeaders(),
    Authorization: `Bearer ${token}`,
  }
}

function readToken(response) {
  try {
    return response.json('data.token') || ''
  } catch (_) {
    return ''
  }
}

function loadTokenFile() {
  const path = (__ENV.SAKU_TOKEN_FILE || '').trim()
  if (!path) return []
  const parsed = JSON.parse(open(path))
  if (!Array.isArray(parsed)) throw new Error('SAKU_TOKEN_FILE must contain a JSON array of JWT strings.')
  return parsed.map((token) => String(token).trim()).filter(Boolean)
}

function buildOptions(profile) {
  const thresholds = {
    http_req_failed: ['rate<0.01'],
    saku_request_errors: ['rate<0.01'],
    http_req_duration: ['p(95)<800', 'p(99)<1500'],
    'http_req_duration{endpoint:health}': ['p(95)<300'],
  }
  if (AUTHENTICATED) {
    thresholds['http_req_duration{endpoint:transactions}'] = ['p(95)<1000']
    thresholds.saku_dashboard_duration = ['p(95)<2500']
  }

  let scenario
  switch (profile) {
  case 'smoke':
    scenario = {
      executor: 'constant-vus',
      vus: envNumber('SAKU_VUS', 2),
      duration: __ENV.SAKU_DURATION || '30s',
    }
    break
  case 'load':
    scenario = {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: __ENV.SAKU_RAMP_UP || '1m', target: envNumber('SAKU_VUS', 25) },
        { duration: __ENV.SAKU_HOLD || '5m', target: envNumber('SAKU_VUS', 25) },
        { duration: __ENV.SAKU_RAMP_DOWN || '1m', target: 0 },
      ],
      gracefulRampDown: '30s',
    }
    break
  case 'stress':
    scenario = {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: stressStages(),
      gracefulRampDown: '30s',
    }
    break
  default:
    throw new Error(`Unknown SAKU_PROFILE "${profile}". Use smoke, load, or stress.`)
  }

  return {
    scenarios: {
      saku: scenario,
    },
    thresholds,
    noConnectionReuse: false,
    userAgent: 'SAKU-k6-load-test/1.0',
    summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
  }
}

function stressStages() {
  if (!envBool('SAKU_ALLOW_STRESS', false)) {
    throw new Error('Stress profile requires SAKU_ALLOW_STRESS=true.')
  }
  const maxVUs = envNumber('SAKU_VUS', 100)
  return [
    { duration: __ENV.SAKU_STRESS_RAMP || '2m', target: Math.max(1, Math.floor(maxVUs * 0.25)) },
    { duration: __ENV.SAKU_STRESS_STEP || '3m', target: Math.max(1, Math.floor(maxVUs * 0.25)) },
    { duration: __ENV.SAKU_STRESS_RAMP || '2m', target: Math.max(1, Math.floor(maxVUs * 0.5)) },
    { duration: __ENV.SAKU_STRESS_STEP || '3m', target: Math.max(1, Math.floor(maxVUs * 0.5)) },
    { duration: __ENV.SAKU_STRESS_RAMP || '2m', target: maxVUs },
    { duration: __ENV.SAKU_STRESS_STEP || '3m', target: maxVUs },
    { duration: __ENV.SAKU_RAMP_DOWN || '2m', target: 0 },
  ]
}

function guardDangerousTargets() {
  const productionTarget =
    BASE_URL.includes('saku.ganipedia.com') ||
    WEB_URL.includes('saku.ganipedia.com') ||
    API_URL.includes('saku.ganipedia.com')
  if (productionTarget && !envBool('SAKU_ALLOW_PRODUCTION', false)) {
    throw new Error(
      'Production target detected. Set SAKU_ALLOW_PRODUCTION=true only after confirming monitoring and test scope.',
    )
  }
}

function envBool(name, fallback) {
  const value = __ENV[name]
  if (value === undefined || value === '') return fallback
  return ['1', 'true', 'yes', 'on'].includes(String(value).toLowerCase())
}

function envNumber(name, fallback) {
  const value = Number(__ENV[name])
  return Number.isFinite(value) && value >= 0 ? value : fallback
}

function randomBetween(min, max) {
  if (max <= min) return min
  return min + Math.random() * (max - min)
}

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, '')
}

function truncate(value, maxLength) {
  const text = String(value || '')
  return text.length > maxLength ? `${text.slice(0, maxLength)}…` : text
}
