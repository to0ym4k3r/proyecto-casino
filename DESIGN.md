# Casino Online — Design System

> Merged from Stake Casino + 1xBet + casino branding best practices (2025).
> Dark-first, premium, energetic. Optimized for Android native (Material Design 3).

---

## 1. Color Palette

### Primary Backgrounds

| Token               | Hex       | Usage                              |
|----------------------|-----------|-------------------------------------|
| `--bg-deep`          | `#0f1923` | App background, deepest layer       |
| `--bg-primary`       | `#1a2c38` | Main surface, cards, containers     |
| `--bg-elevated`      | `#243a4a` | Elevated surfaces, modals           |
| `--bg-surface`       | `#2f4553` | Input fields, borders, dividers     |

### Accent Colors

| Token               | Hex       | Usage                              |
|----------------------|-----------|-------------------------------------|
| `--accent-green`     | `#1db954` | Wins, success, CTAs, active states  |
| `--accent-green-dark`| `#148a3d` | Hover/pressed states for green      |
| `--accent-red`       | `#e74c3c` | Errors, alerts, hot games, losses   |
| `--accent-red-dark`  | `#c0392b` | Hover/pressed states for red        |
| `--accent-blue`      | `#1475E1` | Links, info, game hover overlays    |
| `--accent-gold`      | `#f0b90b` | VIP, promotions, jackpots, premium  |
| `--accent-purple`    | `#9b59b6` | Exclusive content, loyalty tiers    |

### Text Colors

| Token               | Hex       | Usage                              |
|----------------------|-----------|-------------------------------------|
| `--text-primary`     | `#f5f5f5` | Headings, primary content           |
| `--text-secondary`   | `#B1BAD3` | Subtitles, descriptions             |
| `--text-muted`       | `#7a8a9e` | Placeholders, disabled states       |
| `--text-dimmed`      | `#4a5c6e` | Hints, tertiary info                |

### Status Colors

| Token               | Hex       | Usage                              |
|----------------------|-----------|-------------------------------------|
| `--status-success`   | `#27ae60` | Confirmed, approved, online         |
| `--status-warning`   | `#f39c12` | Pending, caution                    |
| `--status-error`     | `#e74c3c` | Failed, blocked, offline            |
| `--status-info`      | `#3498db` | Informational, neutral              |

### Gradients (Premium Touches)

| Name                 | Value                                        | Usage                |
|----------------------|----------------------------------------------|----------------------|
| `--grad-gold`        | `linear-gradient(135deg, #f0b90b, #f9d423)`  | VIP badge, jackpot   |
| `--grad-green`       | `linear-gradient(135deg, #1db954, #2ecc71)`  | Primary CTA button   |
| `--grad-dark`        | `linear-gradient(180deg, #1a2c38, #0f1923)`  | Hero section         |
| `--grad-red-hot`     | `linear-gradient(135deg, #e74c3c, #c0392b)`  | Hot games badge      |

---

## 2. Typography

| Role            | Font Family              | Weight | Size    | Line Height |
|------------------|--------------------------|--------|---------|-------------|
| H1 (Hero)        | Inter / Roboto           | 800    | 32sp    | 1.2         |
| H2 (Section)     | Inter / Roboto           | 700    | 24sp    | 1.3         |
| H3 (Card title)  | Inter / Roboto           | 600    | 18sp    | 1.3         |
| Body             | Inter / Roboto           | 400    | 16sp    | 1.5         |
| Body small       | Inter / Roboto           | 400    | 14sp    | 1.4         |
| Caption          | Inter / Roboto           | 500    | 12sp    | 1.3         |
| Button           | Inter / Roboto           | 600    | 14sp    | 1.0         |
| Label / Tag      | Inter / Roboto           | 700    | 11sp    | 1.0         |

- **Font fallback chain:** Inter → Roboto → system-default
- All headings uppercase on desktop, mixed-case on mobile for readability
- Minimum readable size: 12sp (WCAG 2.1 AA)

---

## 3. Spacing System

Base unit: **4px** (same as Stake)

| Token   | Value | Usage                          |
|---------|-------|--------------------------------|
| `--sp-1`| 4px   | Micro gaps (icon-text)         |
| `--sp-2`| 8px   | Inner padding, tight gaps      |
| `--sp-3`| 12px  | Card inner padding             |
| `--sp-4`| 16px  | Section padding, standard gap  |
| `--sp-6`| 24px  | Between sections               |
| `--sp-8`| 32px  | Major section separation       |
| `--sp-12`| 48px | Page margins, hero spacing     |
| `--sp-16`| 64px | Maximum whitespace             |

---

## 4. Border Radius

| Token          | Value | Usage                         |
|----------------|-------|-------------------------------|
| `--radius-sm`  | 6px   | Badges, tags, small elements  |
| `--radius-md`  | 8px   | Cards, inputs, buttons        |
| `--radius-lg`  | 12px  | Modals, large cards           |
| `--radius-xl`  | 16px  | Bottom sheets, dialogs        |
| `--radius-full`| 9999px| Avatars, pills, search bar    |

---

## 5. Shadows

| Token          | Value                                             |
|----------------|---------------------------------------------------|
| `--shadow-sm`  | `0 1px 3px rgba(0,0,0,0.3)`                       |
| `--shadow-md`  | `0 4px 8px rgba(0,0,0,0.4)`                       |
| `--shadow-lg`  | `0 8px 24px rgba(0,0,0,0.5)`                      |
| `--shadow-glow-green` | `0 0 20px rgba(29,185,84,0.3)`             |
| `--shadow-glow-gold`  | `0 0 20px rgba(240,185,11,0.3)`             |

---

## 6. Component Patterns

### 6.1 Game Card (Core Component)
- **Aspect ratio:** 3:4 (portrait) — same as Stake
- **Border radius:** 8px
- **Background:** `rgba(0,0,0,0.4)` over image
- **Hover effect:** translateY(-4%) + blue overlay (#1475E1, opacity 0.9)
- **Labels:** 
  - "NEW" tag: `--accent-green` left-aligned
  - "HOT" tag: `--accent-red` left-aligned
  - Both use clip-path ribbon shape
- **Shadow on hover:** `0 4px 12px rgba(0,0,0,0.4)`

### 6.2 Buttons

| Type     | Background          | Text    | Border              | Radius   |
|----------|---------------------|---------|---------------------|----------|
| Primary  | `--grad-green`      | White   | None                | 8px      |
| Secondary| `--bg-surface`      | White   | 1px solid --bg-surface | 8px   |
| Outline  | Transparent         | White   | 1px solid --text-muted | 8px   |
| Danger   | `--accent-red`      | White   | None                | 8px      |
| Ghost    | Transparent         | White   | None                | 8px      |

- **Height:** 44px (touch target), 40px compact
- **Font:** 600 weight, 14sp
- **Padding:** 16px horizontal
- **Transition:** background-color 0.3s, transform 0.15s

### 6.3 Search Bar
- **Shape:** Pill (radius: 9999px)
- **Background:** `--bg-surface` (#0f212e equivalent)
- **Border:** 2px solid `--bg-surface`
- **Icon:** Search icon left, 16px, color `--text-muted`
- **Height:** 44px
- **Focus state:** border-color → `--accent-green`

### 6.4 Navigation (Bottom - Mobile)
- **Background:** `--bg-deep` (#0f1923)
- **Height:** 64px
- **Active indicator:** `--accent-green` dot or highlight
- **Icons:** 24px, inactive `--text-muted`, active `--accent-green`
- **Labels:** 10sp, weight 500

### 6.5 Category Slider (Horizontal Scroll)
- **Title:** H3, weight 600, white
- **Cards:** 120px wide, rounded, background image with overlay
- **Arrows:** Pill buttons, border 1px solid `--bg-surface`, icon `--text-secondary`
- **Gap between items:** 8px

### 6.6 Promotional Banner
- **Layout:** 52% text (left) + 48% image (right), flex row
- **Background:** `--bg-primary` or gradient
- **Border radius:** 8px
- **Height:** 224px
- **Text:** Title H2, description body, CTA button

### 6.7 Modals / Bottom Sheets
- **Background:** `--bg-elevated`
- **Border radius:** 16px (top corners on mobile)
- **Backdrop:** rgba(0,0,0,0.7)
- **Transition:** slide-up 0.2s

---

## 7. Animations & Transitions

| Effect             | Duration | Easing                    |
|--------------------|----------|---------------------------|
| Page fade-in       | 0.8s     | ease-out                  |
| Card hover lift    | 0.3s     | ease                      |
| Button press       | 0.15s    | ease-in-out               |
| Modal slide-up     | 0.2s     | cubic-bezier(0.4,0,0.2,1) |
| Skeleton loading   | 1.5s     | ease-in-out (infinite)    |
| Win celebration    | 1.0s     | spring (overshoot)        |

- **No excessive animations** — casino UX must feel fast, not sluggish
- **Skeleton screens** instead of spinners for game loading
- **Haptic feedback** on Android for bet confirmations

---

## 8. Layout Grid

### Mobile (Android - Primary)
- **Columns:** 3 (portrait), 4 (landscape)
- **Gutter:** 8px
- **Margin:** 16px
- **Game grid:** 2 columns, 8px gap

### Tablet
- **Columns:** 4
- **Gutter:** 12px
- **Margin:** 24px
- **Game grid:** 3-4 columns

### Desktop (Admin Panel / Web)
- **Columns:** 12
- **Max width:** 1280px
- **Gutter:** 16px
- **Sidebar:** 240px fixed (collapsible)
- **Game grid:** 5-6 columns

---

## 9. Android-Specific (Material Design 3 Integration)

- **Dynamic color:** Use Material You dynamic color when available (Android 12+)
- **Dark theme:** Follow `isSystemInDarkTheme()` — always dark for casino
- **Bottom navigation:** Material3 NavigationBar (not NavigationRail on phone)
- **Cards:** Material3 Card with elevation 0, custom background color
- **Typography:** Use Material3 type scale mapped to our tokens
- **Shape:** Material3 shape tokens aligned with our radius system
- **Status bar:** Match `--bg-deep`, translucent nav bar

---

## 10. Accessibility

- **Contrast ratio:** Minimum 4.5:1 for text (WCAG AA)
- **Touch targets:** Minimum 44x44dp
- **Screen reader:** All interactive elements have contentDescription
- **Color blind:** Never use color alone to convey information (always pair with icon/text)
- **Responsible gambling:** Deposit limits, self-exclusion, session timers — always visible

---

## 11. Design References

| Source          | What we took                                    |
|-----------------|-------------------------------------------------|
| **Stake Casino** | Dark theme palette, card hover effects, search bar, category sliders, spacing system, game card layout |
| **1xBet**        | Clean layout hierarchy, responsive grid, vibrant accent palette, micro-interactions, visual hierarchy for promotions |
| **Casino best practices** | Red=energy, Green=wins/luck, Gold=VIP, Purple=exclusivity, Dark=trust+focus |
| **Material Design 3** | Android components, dynamic color, bottom nav, accessibility patterns |
