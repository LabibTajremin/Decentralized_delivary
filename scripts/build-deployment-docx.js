// Builds docs/deployment.docx — the Word version of docs/deployment.md, written
// for a developer who has never worked on this project.
//
//   npm install docx
//   node scripts/build-deployment-docx.js docs/deployment.docx
//
// The prose lives here, not in a template: keep it in step with docs/deployment.md
// when that changes. Verified by converting to PDF and reading the pages.

const {
  Document, Packer, Paragraph, TextRun, HeadingLevel, AlignmentType,
  Table, TableRow, TableCell, WidthType, ShadingType, BorderStyle,
  PageBreak, TableOfContents, LevelFormat, convertInchesToTwip,
} = require('docx');
const fs = require('fs');

const ACCENT = '1F6F4A';
const CODEBG = 'F2F3F5';
const INK = '1A1A1A';

// ---------------------------------------------------------------- helpers

const P = (text, opts = {}) => new Paragraph({
  spacing: { after: opts.after ?? 120, line: 276 },
  alignment: opts.alignment,
  children: runs(text),
  ...(opts.bullet ? { numbering: { reference: 'bullets', level: 0 } } : {}),
  ...(opts.number ? { numbering: { reference: 'numbers', level: 0 } } : {}),
});

// A tiny inline markup: **bold**, `code`, and `code` nested inside **bold**.
function runs(text, base = {}) {
  if (Array.isArray(text)) return text;
  const out = [];
  const re = /(\*\*[^*]+\*\*|`[^`]+`)/g;
  let last = 0, m;
  const plain = (t, extra) => out.push(new TextRun({
    text: t, size: 21, bold: base.bold, color: base.color, ...extra,
  }));
  const mono = (t) => out.push(new TextRun({
    text: t, font: 'Consolas', size: 19, color: base.color || '8A3324', bold: base.bold,
  }));
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) plain(text.slice(last, m.index));
    const tok = m[0];
    if (tok.startsWith('**')) {
      // Recurse so backticks inside a bold span still become monospace.
      out.push(...runs(tok.slice(2, -2), { ...base, bold: true }));
    } else {
      mono(tok.slice(1, -1));
    }
    last = m.index + tok.length;
  }
  if (last < text.length) plain(text.slice(last));
  return out;
}

const H1 = (text) => new Paragraph({
  heading: HeadingLevel.HEADING_1,
  spacing: { before: 360, after: 160 },
  children: [new TextRun({ text, bold: true, size: 30, color: ACCENT })],
});

const H2 = (text) => new Paragraph({
  heading: HeadingLevel.HEADING_2,
  spacing: { before: 260, after: 120 },
  children: [new TextRun({ text, bold: true, size: 24, color: INK })],
});

const H3 = (text) => new Paragraph({
  heading: HeadingLevel.HEADING_3,
  spacing: { before: 200, after: 100 },
  children: [new TextRun({ text, bold: true, italics: true, size: 21, color: INK })],
});

// A shaded monospace block. One Paragraph per line, shading on each.
const Code = (lines) => lines.map((line, i) => new Paragraph({
  spacing: { before: i === 0 ? 100 : 0, after: i === lines.length - 1 ? 160 : 0, line: 240 },
  shading: { type: ShadingType.CLEAR, fill: CODEBG, color: 'auto' },
  indent: { left: 180, right: 180 },
  children: [new TextRun({ text: line || ' ', font: 'Consolas', size: 18 })],
}));

// A callout: left-bordered paragraph.
const Note = (label, text) => new Paragraph({
  spacing: { before: 140, after: 160, line: 276 },
  indent: { left: 180 },
  border: { left: { style: BorderStyle.SINGLE, size: 18, space: 12, color: ACCENT } },
  children: [
    ...runs(label, { bold: true, color: ACCENT }),
    new TextRun({ text: '  ', bold: true, size: 21, color: ACCENT }),
    ...runs(text),
  ],
});

const CONTENT_W = 9360; // A4 portrait minus 1" margins, in DXA

function Tbl(headers, rows, weights) {
  const total = weights.reduce((a, b) => a + b, 0);
  const widths = weights.map((w) => Math.round((w / total) * CONTENT_W));
  widths[widths.length - 1] = CONTENT_W - widths.slice(0, -1).reduce((a, b) => a + b, 0);

  const cell = (text, i, head) => new TableCell({
    width: { size: widths[i], type: WidthType.DXA },
    margins: { top: 80, bottom: 80, left: 120, right: 120 },
    shading: head ? { type: ShadingType.CLEAR, fill: ACCENT, color: 'auto' } : undefined,
    children: [new Paragraph({
      spacing: { after: 0, line: 260 },
      children: head
        ? [new TextRun({ text, bold: true, size: 20, color: 'FFFFFF' })]
        : runs(text).map((r) => r),
    })],
  });

  return new Table({
    width: { size: CONTENT_W, type: WidthType.DXA },
    columnWidths: widths,
    borders: {
      top: { style: BorderStyle.SINGLE, size: 4, color: 'C9CDD2' },
      bottom: { style: BorderStyle.SINGLE, size: 4, color: 'C9CDD2' },
      left: { style: BorderStyle.SINGLE, size: 4, color: 'C9CDD2' },
      right: { style: BorderStyle.SINGLE, size: 4, color: 'C9CDD2' },
      insideHorizontal: { style: BorderStyle.SINGLE, size: 4, color: 'DDE0E3' },
      insideVertical: { style: BorderStyle.SINGLE, size: 4, color: 'DDE0E3' },
    },
    rows: [
      new TableRow({ tableHeader: true, children: headers.map((h, i) => cell(h, i, true)) }),
      ...rows.map((r) => new TableRow({ cantSplit: true, children: r.map((c, i) => cell(c, i, false)) })),
    ],
  });
}

const Gap = (after = 160) => new Paragraph({ spacing: { after }, children: [] });

// ---------------------------------------------------------------- content

const children = [];
const push = (...xs) => { for (const x of xs) { if (Array.isArray(x)) children.push(...x); else children.push(x); } };

// --- Cover
children.push(new Paragraph({
  spacing: { before: 1800, after: 120 },
  alignment: AlignmentType.CENTER,
  children: [new TextRun({ text: 'GoKlay', bold: true, size: 72, color: ACCENT })],
}));
children.push(new Paragraph({
  spacing: { after: 400 },
  alignment: AlignmentType.CENTER,
  children: [new TextRun({ text: 'Deployment Guide', size: 40, color: INK })],
}));
children.push(new Paragraph({
  spacing: { after: 100 },
  alignment: AlignmentType.CENTER,
  children: [new TextRun({
    text: 'Everything a developer who has never seen this codebase needs',
    italics: true, size: 22, color: '5A6068',
  })],
}));
children.push(new Paragraph({
  spacing: { after: 1200 },
  alignment: AlignmentType.CENTER,
  children: [new TextRun({ text: 'to get it running on a server, start to finish.', italics: true, size: 22, color: '5A6068' })],
}));
children.push(new Paragraph({
  alignment: AlignmentType.CENTER,
  spacing: { after: 60 },
  children: [new TextRun({ text: 'Repository: github.com/LabibTajremin/Decentralized_delivary', font: 'Consolas', size: 18, color: '5A6068' })],
}));
children.push(new Paragraph({
  alignment: AlignmentType.CENTER,
  children: [new TextRun({ text: 'Branch: claude/goklay-design-system-9z500x', font: 'Consolas', size: 18, color: '5A6068' })],
}));
children.push(new Paragraph({ children: [new PageBreak()] }));

// --- TOC
children.push(H1('Contents'));
const TOC = [
  ['1.', 'What you are deploying', 'The shape of it \u2014 the repository \u2014 where this document stops'],
  ['2.', 'Read this first: what you can deploy today', 'Production is blocked; a demo is fully supported'],
  ['3.', 'Pick a shape', 'One machine or managed pieces \u2014 and why not serverless'],
  ['4.', 'What you need before you start', 'A machine, a domain, Docker, the repository'],
  ['5.', 'Deploy it', 'Clone, configure, bring it up, migrate, seed, check'],
  ['6.', 'Build the three apps', 'The APKs, and the three things that will bite you'],
  ['7.', 'Schedule the dispatch heartbeat', 'Without this, orders reach ready and stop'],
  ['8.', 'Path B: managed pieces', 'Neon or Supabase, a managed Redis, Fly / Railway / Render'],
  ['9.', 'Day two', 'Deploying, rolling back, backups, resetting a demo, logs'],
  ['10.', 'When it does not work', 'Symptom \u2192 cause \u2192 fix'],
  ['11.', 'What this costs', 'One 2 GB VPS, and what would change that'],
  ['A.', 'Appendix: Environment variables', 'Every variable, with an example and what it does'],
  ['B.', 'Appendix: The demo accounts', 'Seven seeded logins across the three apps'],
];
for (const [num, title, sub] of TOC) {
  children.push(new Paragraph({
    spacing: { before: 120, after: 20 },
    indent: { left: 360, hanging: 360 },
    children: [
      new TextRun({ text: num.padEnd(5, ' '), bold: true, size: 21, color: ACCENT, font: 'Consolas' }),
      new TextRun({ text: title, bold: true, size: 21 }),
    ],
  }));
  children.push(new Paragraph({
    spacing: { after: 0 },
    indent: { left: 360 },
    children: [new TextRun({ text: sub, size: 19, color: '5A6068' })],
  }));
}
children.push(new Paragraph({ spacing: { before: 400 }, children: [new TextRun({ text: 'Chapters 4 to 7 are the deployment itself. If you only read one thing, read chapter 2 first \u2014 it says what this can and cannot do today.', italics: true, size: 19, color: '5A6068' })] }));
children.push(new Paragraph({ children: [new PageBreak()] }));

// --- 1. Orientation
children.push(H1('1. What you are deploying'));
children.push(P('GoKlay is a delivery platform for Bangladesh: a customer orders from a shop, a merchant accepts and prepares it, and a delivery partner carries it. This chapter exists so that the rest of the guide makes sense to somebody who has never opened the repository. Skip it if you already know the system.'));

children.push(H2('1.1  The shape of it'));
children.push(P('One Go binary, one Postgres database, one Redis, and three mobile apps. That is the whole system. There is no message broker, no Kubernetes, no microservice mesh, and nothing else to provision.'));
push(Tbl(
  ['Piece', 'What it is', 'Why it is there'],
  [
    ['API', 'One Go 1.25 binary (`cmd/api`), a modular monolith of fourteen modules', 'Every business rule lives here — fees, radii, dispatch, commission, state machines'],
    ['Postgres', 'Postgres 16 with the **PostGIS** and **pg_trgm** extensions', 'All durable state. PostGIS does the radius searches; pg_trgm does shop and dish search'],
    ['Redis', 'Redis 6.2 or newer', 'Sessions, refresh tokens, one-time codes, rate limits. All of it has a TTL; none of it is transactional'],
    ['Caddy', 'Reverse proxy in front of the API', 'TLS certificates, automatically, and the SSE pass-through for live tracking'],
    ['Sweep', 'A tiny shell loop in its own container', 'Calls the dispatch heartbeat every twelve seconds (see chapter 7)'],
    ['Three apps', 'Flutter 3.47.5: `customer`, `merchant`, `partner`', 'They render what the API sends and decide nothing themselves'],
  ],
  [14, 40, 46],
));
children.push(Gap());
children.push(Note('Worth knowing.', 'The apps are deliberately thin. Money crosses the wire as a minor-unit integer **and** a preformatted display string, and every label, notice and status line is composed by the server. That means a pricing or wording change is a backend deploy, not three app-store releases — but it also means the API and the apps must be deployed as a matched pair.'));

children.push(H2('1.2  The repository, in one screen'));
push(Code([
  'backend/          the Go module — one binary, fourteen modules',
  '  cmd/api/        the server',
  '  cmd/migrate/    the migrator: up, down, status, seed',
  '  migrations/     numbered SQL, each with a tested .down.sql',
  '  migrations/seed/  geography, shops, menus, demo accounts',
  'backend/tests/    a separate Go module: unit, integration, e2e, load',
  'frontend/         a Dart pub workspace',
  '  goklay_core/    design system + API client, shared by all three',
  '  customer/  merchant/  partner/',
  'deploy/           what this guide uses: compose file, Caddyfile, .env.example',
  'docs/             deployment.md (this document), demo.md, runbook.md, decisions/',
  'scripts/          verify.sh runs every gate CI runs',
]));

children.push(H2('1.3  Where this document stops'));
children.push(P('This guide covers getting it up. `docs/runbook.md` covers operating it once it is up — probes, incidents, on-call. `docs/demo.md` covers the seeded demo accounts and how to walk somebody through the product. Everything below was run against the real binary, except the container image build itself, which needs a Docker daemon.'));

// --- 2. What you can deploy today
children.push(H1('2. Read this first: what you can deploy today'));
children.push(Note('Production is blocked, by design.', 'One-time codes are the only way anybody signs in, and the only SMS sender that exists writes the code to the application log. The only payment gateway that exists never moves money. Both refuse to be constructed when `APP_ENV=production`, so **the binary will not start**.'));
children.push(P('Someone has to write those two adapters before this can take real money from real people. The seams already exist and each is a single file:'));
children.push(P('`backend/internal/modules/identity/application/ports.SMSSender` — send a code to a phone number.', { bullet: true }));
children.push(P('`backend/internal/modules/payment/application/ports.Gateway` — start a payment and receive its webhook.', { bullet: true }));
children.push(P('**A demo is fully supported**, and is what most people want first. With `DEMO_MODE=true` the one-time code comes back in the response to the request that asked for it, and the app can settle a manual-gateway payment itself. Seven seeded accounts across the three apps come with it — two customers, two merchants, two riders and an admin.'));
children.push(P('So: set `APP_ENV=staging`, and everything in this guide works end to end.'));

// --- 3. Pick a shape
children.push(H1('3. Pick a shape'));
push(Tbl(
  ['', 'A — one machine', 'B — managed pieces'],
  [
    ['What', 'A VPS running Docker Compose: Postgres, Redis, the API, Caddy', 'Neon or Supabase for Postgres, a managed Redis, the API container on Fly.io / Railway / Render'],
    ['Cost', 'One bill, roughly the price of a 2 GB VPS', 'Free tiers cover a demo; three bills later'],
    ['Time', 'About ten minutes', 'About thirty, mostly waiting on dashboards'],
    ['TLS', 'Caddy does it', 'The platform does it'],
    ['Backups', 'Yours to set up', 'Included, with point-in-time restore'],
    ['Best for', 'A demo, a pilot, anything you want to be able to `docker compose logs`', 'Something you expect to grow, or a team that does not want a server'],
  ],
  [12, 44, 44],
));
children.push(Gap());
children.push(P('**Start with A.** It is one `docker compose up -d`, everything is in one place while you are still learning what the product does, and moving to B later is a change of connection strings rather than a rewrite. Chapters 4 to 7 do A in full; chapter 8 gives B’s differences.'));

children.push(H2('3.1  Why not Vercel, or anything serverless'));
children.push(P('Worth stating plainly, because it is the usual first instinct and it does not fit here.'));
children.push(P('**The tracking screen is server-sent events.** `GET /v1/track/{orderId}` holds one response open for the whole delivery. Serverless request timeouts kill it — and a customer watching a rider move is the demo’s best moment.', { bullet: true }));
children.push(P('**The dispatch heartbeat runs every twelve seconds** (chapter 7). That is a process, not a request.', { bullet: true }));
children.push(P('**The API is one long-lived binary** with a Postgres connection pool. Per-request cold starts would open a new pool each time, and a managed Postgres would run out of connections long before traffic did.', { bullet: true }));
children.push(P('A container that stays running is the right shape. Fly.io, Railway, Render and a plain VPS all do that; Vercel and Lambda do not.'));

// --- 4. Prerequisites
children.push(H1('4. What you need before you start'));
children.push(P('**A machine.** 2 GB RAM is comfortable for a demo — Postgres with PostGIS is the hungry part. 1 GB works if nothing else is on it. Any Linux with Docker.', { bullet: true }));
children.push(P('**A domain**, with an `A` record already pointing at that machine. Caddy requests a certificate on first start and the request fails if the name does not resolve there yet, so do this first and let it propagate.', { bullet: true }));
children.push(P('**Docker and the compose plugin.**', { bullet: true }));
push(Code(['curl -fsSL https://get.docker.com | sh']));
children.push(P('**The repository on the machine**, or built elsewhere and pushed to a registry. The simplest thing is to clone it.', { bullet: true }));
children.push(Gap(60));
children.push(P('You do **not** need Go, Flutter, or anything else installed on the server. The image builds both binaries it needs — the API and the migrator — and ships them in a distroless image with nothing else in it.'));

// --- 5. Deploy it
children.push(H1('5. Deploy it'));
children.push(H2('5.1  Clone and configure'));
push(Code([
  'git clone https://github.com/LabibTajremin/Decentralized_delivary.git',
  'cd Decentralized_delivary/deploy',
  '',
  'cp .env.example .env',
]));
children.push(P('`deploy/.env.example` is commented line by line; five values actually matter:'));
push(Code([
  'GOKLAY_DOMAIN=demo.example.com         # the A record you pointed here',
  'TLS_EMAIL=you@example.com              # Let’s Encrypt expiry warnings',
  'PUBLIC_BASE_URL=https://demo.example.com',
  '',
  'POSTGRES_PASSWORD=$(openssl rand -base64 24)',
  'JWT_SIGNING_KEY=$(openssl rand -base64 48)',
  'PAYMENT_WEBHOOK_SECRET=$(openssl rand -base64 48)',
]));
children.push(Note('Generate the signing key even for a demo.', 'Outside production it otherwise defaults to `insecure-development-signing-key-do-not-use`, which is printed in this repository’s source. Without your own, anybody could mint an admin token for your deployment. The signing key is read from the environment and never stored in the database.'));

children.push(H2('5.2  Bring it up'));
push(Code([
  'docker compose up -d --build',
  'docker compose ps          # postgres and redis healthy, api and caddy up',
]));
children.push(P('The first build takes a few minutes: it compiles the API and the migrator into a distroless image. Only Caddy publishes ports (80 and 443) — Postgres, Redis and the API are reachable only from inside the compose network.'));

children.push(H2('5.3  Create the schema'));
children.push(P('The image ships the migrator alongside the API, so this needs no Go on the server. Override the entrypoint:'));
push(Code([
  'docker compose run --rm --entrypoint /migrate api up',
  'docker compose run --rm --entrypoint /migrate api status',
]));
children.push(P('`status` should show twelve migrations applied, nothing pending, not dirty. `migrate up` is safe to run on every deploy: an up-to-date database applies nothing.'));
children.push(P('The first run needs a database role that may `CREATE EXTENSION` — migration `0001` installs PostGIS and `0006` installs `pg_trgm`, both into the `public` schema. The `postgis/postgis` image and both Neon and Supabase allow this by default.'));

children.push(H2('5.4  Load the world'));
children.push(Note('This is not optional, and it is not only demo data.', 'The divisions, districts and areas in `seed/0001_geo.sql` are real geometry the product cannot work without: no areas means no config resolution, no division ceiling, and no delivery fee.'));
push(Code(['docker compose run --rm --entrypoint /migrate api seed']));
children.push(P('Four scripts run: the geography, fourteen approved shops, their menus, and the seven demo accounts. It is idempotent — running it again restores anything that was edited without duplicating anything. Seeding is refused outright when `APP_ENV=production`.'));

children.push(H2('5.5  Check it worked'));
push(Code([
  'curl -fsS https://demo.example.com/healthz   # {"status":"ok"}',
  'curl -fsS https://demo.example.com/readyz    # {"status":"ready"}',
]));
children.push(P('`readyz` is the one that matters: it touches both stores and names the one it could not reach. Then sign in, which is the real end-to-end test:'));
push(Code([
  'curl -sS -X POST https://demo.example.com/v1/auth/otp/request \\',
  '  -H \'Content-Type: application/json\' -d \'{"phone":"01700000001"}\'',
]));
children.push(P('In demo mode the response carries `demo_code`. If it does, the whole chain works — DNS, TLS, Caddy, the API, Postgres, Redis and the seed.'));

// --- 6. Apps
children.push(H1('6. Build the three apps'));
children.push(P('The APKs are built on your machine, not the server, and they are built **against the URL above**, which is compiled in:'));
push(Code([
  'cd frontend',
  'flutter pub get',
  '',
  'for app in customer merchant partner; do',
  '  ( cd "$app" && flutter build apk --release --split-per-abi \\',
  '      --dart-define=GOKLAY_API_BASE_URL=https://demo.example.com )',
  'done',
]));
children.push(P('The APKs land in `frontend/<app>/build/app/outputs/flutter-apk/`. For a phone, `app-arm64-v8a-release.apk` is the one; the others are for older 32-bit devices and for emulators.'));
children.push(H2('6.1  Three things that will bite you'));
children.push(P('**The define name is `GOKLAY_API_BASE_URL`.** Anything else is silently ignored and the app falls back to `http://10.0.2.2:8080`, which is the Android emulator’s route to its host — so a build with a typo works on an emulator and fails on a real phone in a way that looks like a network bug.', { bullet: true }));
children.push(P('**The URL ships inside the APK.** Changing it means a new build, not a config change. Decide the domain before you hand anyone an APK.', { bullet: true }));
children.push(P('**It must be `https`.** Android blocks cleartext by default, so an app built against `http://` fails on every request with no useful error.', { bullet: true }));
children.push(P('Flutter 3.47.5 is what CI pins and what `goklay_core/pubspec.yaml` constrains against. A different version may or may not build.'));

// --- 7. Heartbeat
children.push(H1('7. Schedule the dispatch heartbeat'));
children.push(Note('Without this, orders reach `ready` and stop.', 'There is no background worker inside the API: something has to call `POST /v1/admin/dispatch/sweep`, which expires offers nobody answered and offers waiting jobs to somebody. When it is missing, the symptom is a rider with an empty feed and a customer whose food never moves — and it looks like a bug in the product rather than a missing cron.'));
children.push(P('The compose file already runs it: the `sweep` service, every twelve seconds. `dispatch.assignment_timeout` is thirty seconds, so a slower cadence adds its own interval to every reassignment.'));
children.push(P('It has to sign in, which is the awkward part, because the admin surface needs a token and tokens come from one-time codes. `deploy/sweep.sh` handles both cases:'));
children.push(P('**On a demo**, it signs itself in with `ADMIN_PHONE` and the revealed code. Nothing to set up.', { bullet: true }));
children.push(P('**Anywhere else**, it uses a refresh token from `/state/refresh`, rotating and rewriting it on every use. Put the first one there by hand:', { bullet: true }));
push(Code([
  '# sign in as your admin once, however you normally would, then:',
  'printf \'%s\' \'the-refresh-token\' \\',
  '  | docker compose exec -T sweep sh -c \'cat > /state/refresh\'',
  'docker compose restart sweep',
]));
children.push(P('Refresh tokens last sixty days and rotate on every use, so the job keeps itself signed in indefinitely as long as it keeps that file. The volume it lives on survives restarts.'));
children.push(P('Check it is working:'));
push(Code([
  'docker compose logs sweep | tail',
  '# sweep: starting: every 12s against http://api:8080',
  '# sweep: signed in as 01700000031',
]));
children.push(P('Silence after those two lines is success — it only logs problems.'));

children.push(H2('7.1  Raise the OTP rate limit on a demo'));
children.push(P('`auth.otp_requests_per_hour` is five per number per hour. That is right when a number belongs to one person and far too low when everybody shares seven demo numbers. Raise it to its maximum, as the admin:'));
push(Code([
  'PUT /v1/config/overrides',
  '{"key":"auth.otp_requests_per_hour","level":"global","code":"",',
  ' "value":"20","reason":"shared demo numbers"}',
]));
children.push(P('An ordinary admin setting — not something demo mode changes.'));

// --- 8. Path B
children.push(H1('8. Path B: managed pieces'));
children.push(P('Everything above holds; three things change.'));
children.push(H2('8.1  Postgres — Neon or Supabase'));
children.push(P('Both are Postgres with PostGIS available, and both free tiers are ample for a demo. Neon’s branch-per-environment is genuinely useful here: a staging branch of the demo database costs nothing and resets in seconds.'));
children.push(P('Take the connection string, **add `sslmode=require`**, and use it as `DATABASE_URL`. Run the migrator against it from anywhere that can reach it:'));
push(Code([
  'docker run --rm -e DATABASE_URL=\'postgres://…?sslmode=require\' \\',
  '  --entrypoint /migrate goklay-api:latest up',
]));
children.push(P('Watch the connection limit: each API replica holds its own pool, and free tiers are stingy. One replica is fine; before scaling out, check the ceiling.'));
children.push(H2('8.2  Redis — any managed Redis'));
children.push(P('Sessions, refresh tokens and rate limits live there, and nothing transactional does, so losing it signs everybody out and costs nobody an order. Two requirements to check on whatever you pick: the rate limiter runs a small **Lua script** (`EVAL`), and the OTP store uses **`GETDEL`**, which needs Redis 6.2 or newer. Use `rediss://` where TLS is offered.'));
children.push(H2('8.3  The API — Fly.io, Railway or Render'));
children.push(P('All three run a container that stays running, terminate TLS for you, and take the same environment variables. Drop the `postgres`, `redis` and `caddy` services; keep `api`, and keep `sweep` somewhere that can reach it — Fly’s `[processes]`, a Railway service, or a Render cron job at its finest granularity.'));
children.push(P('You will not need the Caddyfile. You will still need every secret from chapter 5.'));
children.push(Note('One thing the platform must not do.', 'Whatever sits in front of the API must not buffer responses. The supplied Caddyfile sets `flush_interval -1` for `/v1/track/*`; a proxy that buffers turns live tracking into a screen that never updates.'));

// --- 9. Day two
children.push(H1('9. Day two'));
children.push(H2('9.1  Deploying a new version'));
children.push(P('Migrations here are additive, so the order is **schema first, then code**: the old binary keeps working against the new schema, and there is no window where a half-replaced deployment is broken.'));
push(Code([
  'git pull',
  'docker compose run --rm --entrypoint /migrate api status',
  'docker compose run --rm --entrypoint /migrate api up',
  'docker compose up -d --build api',
]));
children.push(P('`SIGTERM` starts a graceful shutdown bounded by `SHUTDOWN_TIMEOUT` (ten seconds), and in-flight requests finish — `TestGracefulShutdown` asserts it against the real binary, so a redeploy does not drop somebody’s order.'));

children.push(H2('9.2  Rolling back'));
children.push(P('Roll the **code** back first; it is the safe half.'));
push(Code([
  'git checkout <previous tag>',
  'docker compose up -d --build api',
]));
children.push(P('Rolling the schema back is a decision, not a step. Every migration has a tested `down` — the integration suite rolls the whole schema back and forward on every run — but they still drop columns, and a `down` after real traffic destroys whatever the new version wrote. Restore from a backup instead, unless the migration was minutes old.'));

children.push(H2('9.3  Backups'));
push(Code([
  'docker compose exec -T postgres \\',
  '  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > goklay-$(date +%F).sql.gz',
]));
children.push(P('Put that in cron and copy it off the machine. **Test a restore before you need one** — point a staging API at it and run `migrate status`. Redis needs no backup: it is a TTL store by design.'));

children.push(H2('9.4  Resetting a demo'));
children.push(P('Visitors leave orders, reviews and tickets behind. The seed restores the accounts and shops but removes nothing, so a clean slate is a rebuild:'));
push(Code([
  'docker compose run --rm --entrypoint /migrate api down 0',
  'docker compose run --rm --entrypoint /migrate api up',
  'docker compose run --rm --entrypoint /migrate api seed',
  'docker compose exec redis redis-cli FLUSHALL',
  'docker compose restart sweep    # its stored refresh token just died',
]));
children.push(P('Nightly is a reasonable default for a demo strangers can reach.'));

children.push(H2('9.5  Logs'));
push(Code(['docker compose logs -f api']));
children.push(P('One structured JSON line per request: method, path, status, bytes, duration and `request_id` — which is echoed in the `X-Request-Id` response header. That is what turns “a customer says it failed at 14:32” into one query. No bodies, no `Authorization` header, no query strings.'));
children.push(P('Alert on `level=ERROR`: client errors log at info and server errors at error, so it fires for real bugs and not for somebody sending a bad postcode.'));

// --- 10. Troubleshooting
children.push(H1('10. When it does not work'));
push(Tbl(
  ['Symptom', 'Almost always', 'Fix'],
  [
    ['Caddy loops on “obtaining certificate”', 'The domain does not resolve to this machine yet', 'Check the `A` record; wait for propagation; `docker compose restart caddy`'],
    ['`readyz` says `database`', 'Postgres is not up, or `DATABASE_URL` is wrong', '`docker compose ps`; on a managed Postgres check `sslmode=require`'],
    ['`readyz` says `redis`', 'Same, for Redis', 'Sign-in is down; everything already authenticated keeps working ~15 minutes'],
    ['API exits immediately', 'A required variable is missing or invalid', '`docker compose logs api` — it names the variable'],
    ['`DEMO_MODE must not be set in production`', '`APP_ENV=production` with `DEMO_MODE=true`', 'Use `staging`'],
    ['`the logging SMS sender must not be used in production`', '`APP_ENV=production` with no SMS adapter', 'Chapter 2 — this is the release blocker, not a misconfiguration'],
    ['`migrate up` fails on `CREATE EXTENSION`', 'The role cannot install extensions', 'Use a role that can, or install PostGIS and `pg_trgm` once by hand'],
    ['No `demo_code` in the response', '`DEMO_MODE` is not true', 'Check it reached the container with `docker compose config` (there is no shell in the image)'],
    ['`429` on requesting a code', 'Five per number per hour', 'Chapter 7.1, raise it — or wait'],
    ['Orders reach `ready`, no rider is offered', 'The sweep is not running', '`docker compose logs sweep`'],
    ['The app cannot reach the API at all', 'Built with the wrong define, or with `http://`', 'Rebuild with `--dart-define=GOKLAY_API_BASE_URL=https://…`'],
    ['The tracking screen never updates', 'A proxy is buffering the SSE stream', 'The supplied Caddyfile sets `flush_interval -1` for `/v1/track/*`; any replacement proxy needs the equivalent'],
  ],
  [30, 32, 38],
));

// --- 11. Costs
children.push(H1('11. What this costs'));
children.push(P('For a demo: **one 2 GB VPS.** Postgres, Redis, the API and Caddy fit comfortably; the three APKs cost nothing to host because you hand them out as files. That is the whole bill.'));
children.push(P('Path B’s free tiers cover a demo too — Neon and Supabase both have one, and so does every managed Redis worth using — but you will be watching three dashboards instead of one machine, and the free Postgres tiers sleep when idle, which makes the first request after a quiet night slow.'));
children.push(P('What would change the arithmetic, roughly in the order you would hit it:'));
children.push(P('**Postgres CPU.** The two spatial queries are the only compute-heavy path. `docs/load-test.md` has measurements: at two thousand shops and two thousand riders, a radius search is about 1.4 ms and a candidate pool about 8.5 ms, both index-assisted and both gated so they stay that way.', { number: true }));
children.push(P('**Concurrent deliveries**, not requests per second. Each customer watching a rider holds one connection open for the length of the delivery.', { number: true }));
children.push(P('**API replicas**, which multiply Postgres connections. Add them behind Caddy or the platform’s load balancer; the binary is stateless and holds nothing locally.', { number: true }));
children.push(P('Nothing here needs a Kubernetes cluster, and putting one in front of it would cost more than the product does.'));

// --- Appendix
children.push(new Paragraph({ children: [new PageBreak()] }));
children.push(H1('Appendix A. Environment variables'));
children.push(P('Every one of these is read once when a container starts, so changing a value means `docker compose up -d` again. None of them is a business rule: radii, fees, COD limits and commission are tuned at runtime by an admin and live in the config module.'));
push(Tbl(
  ['Variable', 'Example', 'Notes'],
  [
    ['`GOKLAY_DOMAIN`', '`demo.example.com`', 'Its `A` record must already point here, or Caddy cannot get a certificate'],
    ['`TLS_EMAIL`', '`you@example.com`', 'Where Let’s Encrypt sends expiry warnings'],
    ['`PUBLIC_BASE_URL`', '`https://demo.example.com`', 'The same name as the apps see it. Must be `https`'],
    ['`APP_ENV`', '`staging`', '`development` | `staging` | `production`. Production will not start — chapter 2'],
    ['`DEMO_MODE`', '`true`', 'Reveals one-time codes and allows in-app payment simulation. Refused with `production`'],
    ['`ADMIN_PHONE`', '`01700000031`', 'The seeded admin the sweep signs in as on a demo'],
    ['`POSTGRES_USER` / `_PASSWORD` / `_DB`', '`goklay` / … / `goklay`', 'Generate the password: `openssl rand -base64 24`'],
    ['`JWT_SIGNING_KEY`', '—', '**Generate it**: `openssl rand -base64 48`. Never stored in the database'],
    ['`PAYMENT_WEBHOOK_SECRET`', '—', 'Generate it the same way'],
    ['`JWT_ISSUER`', '`goklay`', 'The `iss` claim, so a token from one environment is refused by another'],
    ['`CORS_ALLOWED_ORIGINS`', '(empty)', 'Empty is right for a mobile-only deployment; the apps are native'],
    ['`LOG_LEVEL`', '`info`', 'Client errors log at info, server errors at error'],
    ['`TRACKING_STREAM_INTERVAL`', '`3s`', 'How often a tracking stream polls. Lower feels faster and costs a query per open stream per tick'],
    ['`SWEEP_INTERVAL`', '`12`', 'Seconds between dispatch sweeps. Above ~15 adds its own interval to every reassignment'],
  ],
  [31, 21, 48],
));

children.push(H1('Appendix B. The demo accounts'));
children.push(P('Loaded by `seed/0009_demo_accounts.sql`. In demo mode, request a code for any of these numbers and the response carries it. Full walkthroughs are in `docs/demo.md`.'));
push(Tbl(
  ['Phone', 'Role', 'What it is for'],
  [
    ['`01700000001`', 'Customer', 'Dhanmondi address — the main ordering walkthrough'],
    ['`01700000002`', 'Customer', 'Mirpur address — a second area, for fees and radius behaviour'],
    ['`01700000011`', 'Merchant', 'Owns the demo restaurant (`MER-DEMO-0001`), open 00:00–24:00'],
    ['`01700000012`', 'Merchant', 'Owns the demo grocery (`MER-DEMO-0005`), open 00:00–24:00'],
    ['`01700000021`', 'Delivery partner', 'Registered and positioned, off shift — go on shift in the app'],
    ['`01700000022`', 'Delivery partner', 'The second rider, for reassignment and the offer timeout'],
    ['`01700000031`', 'Admin', 'Config overrides, dispatch sweep, approvals'],
  ],
  [20, 22, 58],
));
children.push(Gap());
children.push(Note('A demo is a deployment where anyone can sign in as anyone.', 'Do not point one at anything you care about, do not reuse its secrets anywhere else, and reset it on a schedule (chapter 9.4).'));

// ---------------------------------------------------------------- document

const doc = new Document({
  creator: 'GoKlay',
  title: 'GoKlay Deployment Guide',
  description: 'How to deploy GoKlay, for a developer new to the project.',
  numbering: {
    config: [
      {
        reference: 'bullets',
        levels: [{
          level: 0, format: LevelFormat.BULLET, text: '•', alignment: AlignmentType.LEFT,
          style: { paragraph: { indent: { left: convertInchesToTwip(0.3), hanging: convertInchesToTwip(0.18) } } },
        }],
      },
      {
        reference: 'numbers',
        levels: [{
          level: 0, format: LevelFormat.DECIMAL, text: '%1.', alignment: AlignmentType.LEFT,
          style: { paragraph: { indent: { left: convertInchesToTwip(0.3), hanging: convertInchesToTwip(0.18) } } },
        }],
      },
    ],
  },
  styles: {
    default: {
      document: { run: { font: 'Calibri', size: 21, color: INK } },
    },
  },
  sections: [{
    properties: {
      page: { margin: { top: 1080, bottom: 1080, left: 1080, right: 1080 } },
    },
    children: children.filter(Boolean),
  }],
});

Packer.toBuffer(doc).then((buf) => {
  fs.writeFileSync(process.argv[2], buf);
  console.log('wrote', process.argv[2], buf.length, 'bytes');
});
