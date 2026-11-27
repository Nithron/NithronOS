# nOS web interface: the Nithron identity

Status: approved by Thiago on 2026-11-27 for phase 1 (foundations), on
branch `ui/identity`. It follows the website work's chapter for
applications (nithron-site, `DESIGN-APPS.md`, whose note "Settled on
2026-11-27, evening" wins over the rest of it). Each phase is reviewed by
the website session against that chapter before the next starts; nothing
is merged to main without Thiago's word.

## Settled (2026-11-27, evening)

These decisions override what the proposal below says:

- **No hue.** The interface has no main colour. The current item, the
  focus ring (an ink outline), underlines, a running lamp and chosen
  controls are ink or grey. Wherever the text below says `--main`, read
  ink.
- **Status colours only.** Green for in order, red with a cross for
  failed, gold with an open square for a warning.
- **No drawings for now.** Empty screens, sign-in and setup are words and
  one action.
- **Icons.** One shared pixel icon set, drawn by the website session for
  all three products, on a 16 px grid, shown at 16 or 32 px. Until it
  arrives every icon goes through `web/src/components/icons.ts`, named by
  meaning.
- **Tokens and fonts.** `@nithron/tokens` is under the Nithron Community
  License. `line.css` 0.2.0 is copied whole into
  `web/src/styles/nithron-tokens.css` and is replaced whole on a new
  release (0.3 brings soft grounds and a neutral `--main`). The fonts ship
  in `web/public/fonts`.
- **Answers to the questions at the end:**
  1. The theme follows the system, with a setting through the appearance
     API that pins it.
  2. No drawings.
  3. The foundations don't wait for a colour.
  4. Licence settled.
  5. Destructive actions are line buttons with the typed confirmation in
     nOS's own dialog.

**Phase 1, done on the branch:**

- the tokens and fonts;
- the shadcn names mapped onto the tokens (`web/src/styles.css`);
- Tailwind colours through `color-mix`;
- the theme setting (admin-only);
- icons by meaning;
- `npm run check:ui`;
- the live walk in both themes, with captures and an AA contrast report.

**Phase 2:** the shell, then the shared components. **Then** the screens,
area by area, storage first.

## The interface today (checked against the code)

**Screens.** These are the routes in `web/src/App.tsx`. The live walk
(`tests/e2e/ui/tests/live-walk.spec.ts`) opens every one of them against a
real nosd.

| Area | Screens |
|------|---------|
| Outside the app | sign-in, first-boot setup (code, admin, two-factor, done), proxy help, not found |
| Dashboard | the dashboard |
| Storage | pools, create pool (devices, name, profile, plan, confirm), pool details, disk health, system health |
| Shares | list, new share, share details (permissions, SMB, NFS, sync) |
| Apps | catalog, install wizard, installed app details (logs, events, snapshots) |
| Backup | snapshots, schedules, remote destinations |
| Monitoring | charts, alerts |
| Settings | settings, users, groups, schedules, updates, network, remote-access wizard, two-factor, NithronSync (devices and pairing QR, activity, conflicts, sharing, encryption) |

**How it is styled.**

- **Stack.** React 18, Vite, Tailwind 3 and shadcn-style components on Radix
  (`web/src/components/ui/`). One shell: `components/layout/AppShell.tsx`.
  Navigation uses react-router `NavLink`, so the current item already has
  `aria-current="page"`.
- **Colours.** HSL triplets in `web/src/styles.css`, read as `hsl(var(--x))`
  in `tailwind.config.ts`. 64 classes add an opacity to a theme colour, for
  example `bg-primary/10`, `border-muted/30` and `hover:bg-primary/90`.
- **Dark only.** `:root` and `.dark` hold the same values, and nothing ever
  sets `.dark`, so the 23 `dark:` classes never apply. The appearance API
  (`/api/v1/settings/appearance`) exists, but the UI doesn't use it.
- **Raw palette classes.** 488 of them in 40 files, and status colours are
  chosen page by page. Problems these cause today:
  - **Dead blue classes.** The Tailwind config defines `cyan`, `blue` and
    `lime` as single values, which replaces Tailwind's scales of those names.
    The 86 classes such as `text-blue-500` produce no CSS at all; they are
    dead today.
  - **Monitoring chart colours.** Three monitoring series read `--chart-2`,
    `--chart-3` and `--chart-4`. Nothing defines those variables.
- **Toasts: three mechanisms.** sonner (3 files), react-hot-toast (2) and
  the app's own `components/ui/toast.tsx` (24).
- **Loading.** 27 skeletons, 30 spinning icons, 6 pulses.
- **Decoration.** 25 shadows, 6 gradients, 2 blurs.
- **Motion.** framer-motion lists animate their rows in on 5 pages
  (dashboard, monitoring, app catalog, disk and system health).
- **Charts.** recharts: a storage donut on the dashboard and area/line charts
  in monitoring.
- **Type and drawings.** No font set (the system's). Icons are lucide at its
  default 2 px stroke. No drawings.
- **QR codes.** Device pairing and two-factor setup draw QR codes, dark on
  white.
- **Password strength.** The setup's meter uses five colours, red to green.

**Other nOS interfaces in this repository.**

- The NithronSync desktop client (`clients/desktop`: Wails, React,
  framer-motion, lucide).
- The mobile client (`clients/mobile`: React Native, which has no CSS
  variables).
- The boot menu and installer branding (`packaging/iso`).
- The documentation site (`docs-site`, mkdocs-material).

The chapter covers none of these. This proposal covers the web interface;
they follow once it is approved.

## What the chapter means for nOS

### Foundations (one pull request, nothing visible changes yet)

- `web/` installs the packed `@nithron/tokens` archive and imports `line.css`
  before `styles.css`. `styles.css` becomes the one mapping file: the shadcn
  names point at the tokens as in the chapter's table.
- **The Tailwind config can't say `background: 'var(--page)'`.** Tested:
  Tailwind 3 produces nothing for `bg-x/10` when the colour is a bare
  variable, so the 64 opacity classes would vanish. Each colour is written
  instead as
  `color-mix(in srgb, var(--page) calc(<alpha-value> * 100%), transparent)`.
  That form generates them all, and it also gives the hover ("ink at 9
  percent") as `bg-ink/[.09]`.
- **Status names.** Status gets application names in the same file:
  `--status-ok`, `--status-warning` and `--status-failed`, read from
  `--status-pass`, `--status-earlier` and `--status-failed`. "Earlier" is
  the website's word, and a degraded pool is not earlier.
- **Theme.** The theme follows the system. `data-theme` on `<html>` comes
  from the appearance setting, if Thiago wants one (question 1).
- **Fonts.** The complete upstream Mona Sans and Monaspace Neon files,
  unchanged, go in `web/public/fonts` with their licence texts and a line in
  `NOTICE`. They are served by nosd, never from another server.
- **Lint rule.** A lint rule refuses raw palette classes, hex values, `dark:`
  variants, `shadow-*`, `bg-gradient-*` and `blur-*` in `web/src`. Each area
  is added to the rule once it has been converted.
- **The live walk** gets a light pass, an AA contrast check of every text on
  every screen in both themes, and a capture of each screen, with one browser
  at a time.

### The shell (one pull request)

- **Ground and rules.** One ground, black or white. The sidebar and the top
  bar have no fill of their own and are parted from the content by a
  hairline.
- **Navigation.** Navigation is words with their line icon beside them
  (lucide at 1.5 px). The sidebar never collapses to bare icons. The current
  item is `--ink`, weight 560, with a 2 px mark in `--main`; the others are
  `--ink-2`.
- **Head of the navigation.** The name "nOS", set in type with its
  lower-case n, stands at the head of the navigation, with one link to
  nithron.com under "About". The drawn mark in `public/brand` waits for the
  identity step and Thiago's approval.
- **No family bar on a NAS.** The chapter lets a deployment show the family
  bar through a setting that is off by default. nOS doesn't need that
  setting: an owner's NAS never shows the bar. The documentation site shows
  it once Nithron hosts it.
- **Focus ring.** The focus ring follows the chapter: an ink outline with a
  `--main` ring outside it.

### Shared components (one pull request)

- **Buttons.** The first action is ink with page-coloured text; the second is
  a line button. "destructive" stops being a red fill and becomes a line
  button whose words say what is destroyed. The typed confirmations stay
  ("type the pool name"), in the dialog. The 7 browser `confirm()` boxes (on
  the two-factor, backup schedules, updates, snapshots and network pages)
  become the product's own dialog.
- **Secrets shown once.** The two-factor backup codes and the NithronSync
  pairing token stand in mono in a field. Each has a Copy button with its
  word, under a sentence saying they will not be shown again, and never in a
  toast.
- **Logs that arrive.** App logs and update logs are a ruled mono region
  that grows at the bottom, with a "Follow" switch. They are not announced
  line by line: only the task's state is (see the note to the website
  session).
- **Fields.** 44 px high, an `--edge` line that turns ink on focus. Values
  are in the mono face. An error stands at its field (cross, words,
  `aria-invalid`).
- **Switches and chips.** The Radix switches (SMB on, sync on, and others)
  become the chapter's switch: an `--edge` track, ink when on, and a label
  that says what is on. The user and group pickers of share permissions
  become chips.
- **Tables.** Ruled grids with no card around them. Heads are labels in
  `--ink-2`. Numbers are in mono and right-aligned. Controls inside a row are
  32 px high.
- **Panels.** Card becomes a ruled panel: square, no fill, no shadow. Dialogs
  and menus have a 1 px `--edge` line, no shadow, and a 70 percent ground
  behind them.
- **Status.** A status is a mark and a word:
  - a filled square for in order;
  - an open square for warning;
  - a cross for failed;
  - a lamp in `--main` for running;
  - `--dim` for off or unknown.

  StatusPill becomes the chapter's pill, with 3 px corners.
- **Toasts.** One toast: the app's own `toast.tsx`, which most pages
  already use. sonner and react-hot-toast leave. A failure stays until it is
  dismissed and is also shown where it happened.
- **Loading.** The screen's ruled layout appears at once, with mono words and
  a blinking cursor where the data will stand. Skeletons and spinners go.
- **Motion.** 150 ms state changes. Rows no longer animate in. Everything
  stops under reduced motion.
- **QR codes.** They stay dark on white with their quiet zone in both
  themes: the one white block on a black page. Scanners fail on inverted
  codes.

### Screen by screen

**Outside the app**

- **Sign-in.** One column: the form (44 px fields, the first action in ink)
  beside a character drawing of nOS as a box with four drive bays. The
  two-factor code field is in mono.
- **First-boot setup.** The four steps as a list, the current one marked in
  `--main`, with the same drawing; at "done", the box with its lamp lit. The
  setup code and the backup codes are in mono. The two-factor QR code is
  white, as above. The password meter becomes a bar of characters with its
  word: failed colour for too weak, pass colour for strong, no five-colour
  ramp.
- **Proxy help, not found.** Text only.

**Dashboard.** Equal ruled cells, with no drawing on this screen: it has
data.

- Top row: four quarters (CPU, memory, storage, uptime), each with one
  number set large and its label.
- Then halves for activity and alerts.
- The storage donut becomes a bar of characters with the used and free
  numbers as text. The chapter has no round charts.

**Storage**

- **Pools.** A ruled table: name, profile, size and used in mono, then
  status as mark and word.
- **No pool yet.** The line figure of the box with empty bays, one sentence,
  and "Create a pool".
- **Create pool.** The steps as a list:
  - **Devices.** A table with checkboxes. A disk in use is `--dim` and says
    why.
  - **Plan.** The agent's steps, one per line, in mono. Each step that
    erases a disk says so in words, with no colour: a status colour gives
    the state of a thing, never the weight of an action.
  - **Confirm.** A line button, for example "Erase 2 disks and create
    pool-1", and the typed name.
  - **Progress.** A phase list: the current step in `--main` with a lamp,
    and a failed step crossed with its output under it.
- **Pool details.** Facts in mono. A scrub is a bar of characters with its
  percentage.
- **Disk and system health.** Tables. A failing disk is the one coloured
  thing on the screen: its row gets a cross and its device name in the
  failed text colour.

**Shares.** A table, and a one-column form for a new share. The details
tabs become the chapter's switch (a radio frame). With no share yet: a
drawing and "Create a share".

**Apps**

- **Catalog.** A ruled grid of equal cells: name, category with its line
  icon, one line of text. The catalog shows category icons today, not the
  apps' own logos (see the note to the website session).
- **Install wizard.** One-column forms.
- **App details.** Logs and events in mono. Status as mark and word.

**Backup.** Snapshots and schedules as tables, remote destinations as a
form. With nothing yet: one drawing.

**Monitoring**

- **Charts.** The subject series is `--main`. The others are told apart by
  line style and a name at the line's end, not by more hues. Network in and
  out become one solid line and one dashed. Axes are hairlines and tick
  labels `--dim`.
- **Alerts.** A table. An alert's severity is the state of the thing it is
  about (a disk failing), so it takes a status mark and a word.
- The undefined `--chart-*` variables go away.

**Settings**

- **Users, groups, schedules.** Tables.
- **Updates.** A phase list (preflight, configuration saved, install, done),
  the current phase in `--main`, a failure crossed with what to do. Rollback
  is a line button, "Restore the configuration from <date>". Its dialog says
  that nosd restarts and packages stay as they are; today it wrongly says
  the system restarts.
- **Network, remote access, two-factor.** One-column forms. The two-factor
  QR code is white.
- **NithronSync.**
  - Devices: a table, plus the pairing QR code (white).
  - Activity and conflicts: tables.
  - Encryption: a form.

## Where the chapter didn't fit nOS

These points were sent to the website session and are now in
`DESIGN-APPS.md` (committed locally there, 2026-11-27):

- **Tailwind 3 mapping.** The chapter now uses the tested `color-mix` form.
- **Neutral notices.** A neutral notice has no hue: ink words, its own
  mark, and its action as a line button.
- **Status names.** The tokens keep the sites' names. nOS names the states
  in its mapping file (ok, warning, failed).
- **QR codes.** Black on white with their quiet zone, in a hairline frame,
  in both themes: the stated exception to one ground.
- **Other surfaces.** The desktop client loads `line.css`. The mobile
  client takes the tokens as data and the theme from the platform. The boot
  screen and installer show the ground and the name in type, with two
  colours at most. The documentation site maps its theme onto the tokens
  and carries the family bar when Nithron hosts it.
- **Third-party app logos.** They count as colours that are data: their own
  colours, a small fixed size, never recoloured.
- **Family bar.** No setting needed: nOS on a NAS never shows it, and the
  hosted documentation always does.
- **Inventory.** The nOS line of the chapter is corrected.

- **Streaming logs.** They are not announced line by line
  (`aria-live="off"`); only the task's state is announced.

## Order

One pull request each, each reviewed before the next:

1. Foundations.
2. The shell.
3. The shared components.
4. The screens, area by area: setup and sign-in first (everyone sees them),
   then dashboard, storage, shares, apps, backup, monitoring, settings.

After that come the desktop client, the mobile client, the boot menu and
the documentation site.

## Questions for Thiago

1. **Theme setting.** The theme follows the system. Should there also be a
   setting (system, light, dark) through the appearance API? Recommended:
   yes. A NAS page often stays open on a screen that isn't the owner's
   usual one.
2. **Drawings.** The chapter limits drawings to empty screens, setup,
   sign-in and the end of a long task (none on the dashboard). Is that
   right for nOS?
3. **When to start.** Can the foundations start before the main colour is
   chosen? nOS only reads `--main`, so the choice changes nothing in nOS
   code.
4. **The tokens' licence.** The package says UNLICENSED and its archive
   would carry the site's MIT file, inside nOS, which is public under the
   NCL. Which licence should the vendored copy carry?
5. **Destructive actions.** A line button with exact words, instead of a red
   fill. Is that right for actions that erase disks?
