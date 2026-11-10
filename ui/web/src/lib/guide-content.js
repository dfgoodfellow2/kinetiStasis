// Plain-language guide content for the in-app manual.
//
// Every number in this file is sourced from the Go implementation so the
// guide cannot drift away from what the app actually does:
//   - internal/constants/constants.go (gating, floors, readiness alpha)
//   - internal/services/readiness/readiness.go (z-scores, escalation zones)
//   - internal/services/nutrition/macros.go (protein/fat/carb/fiber/water)
//   - internal/services/calculator/adaptive.go (tiering, damping, trend)
//   - internal/services/met/met.go + exercise/volume.go (MET-based calories,
//     RPE-driven intensity; MWV/NDS/density stay export-only, not UI displays)
//   - internal/services/bodyfat/bodyfat.go (Navy formulas, Katch-McArdle)
//
// Body entries are deliberately renderer-friendly:
//   'a paragraph'                → <p>
//   { list: ['…'] }              → bullet list
//   { steps: ['…'] }             → numbered list
//   { terms: [{ term, def }] }   → definition list
//   { tip: '…' }                 → highlighted callout
//
// Keep the surface copy warm and plain. Exact mechanics belong in
// underTheHood, where the owner can fold them open if they want them.

export const guideSections = [
  {
    id: 'getting-started',
    title: 'Getting started',
    icon: '🚀',
    body: [
      'This app is a quiet daily habit, not a diet you have to be perfect at. The more consistently you log, the better it can read your body and adjust for you.',
      'The daily loop is simple:',
      {
        list: [
          'Log your weight, water, and food every day. That’s the core — a couple of minutes.',
          'Add grip strength and BOLT if you can. They’re optional, but they’re what power your daily readiness signal.',
          'Every few days, a weekly check-in looks at your data and nudges your calorie targets. You never have to guess the next move.'
        ]
      },
      {
        tip: 'Missing a day is not a failure. The app is built to keep going; it just needs a few honest data points to work with.'
      },
      'Accounts on this instance are invite-only by default: the household owner mints a single-use code, and you register with it. If the owner ever opens registration for a wider launch, new accounts can be set to wait for their approval — the admin manages the registration mode, invite codes, and that approval queue from the 🛠 Admin page.',
      {
        tip: 'You can change your password any time on the Profile page. If you forget it, ask the household owner for a one-time reset — they’ll give you a temporary password you can replace after signing in. Two-factor authentication is opt-in per account from the Profile page; recovery codes are stored hashed and single-use, and a one-time owner reset exists if everything is lost.'
      }
    ],
    underTheHood: [
      'Daily nutrition logs feed the observed-TDEE estimate. Biometric logs feed the weight trend and readiness.',
      'Sessions use httpOnly cookies with SameSite=Lax: the app and API share one origin, so the browser withholds the session on cross-site state-changing requests.',
      {
        list: [
          'Check-ins unlock after at least 5 days (MinCheckinIntervalDays) and need at least 3 weigh-ins; targets only move once 7 days of weigh-ins make the weight term reliable.',
          'Daily calorie targets never fall below 1200 kcal (MinCalorieFloor).',
          'Readiness uses the profile’s gripWeight to blend grip and BOLT; the default is 50/50.'
        ]
      }
    ]
  },
  {
    id: 'trend-weight',
    title: 'Trend weight',
    icon: '⚖️',
    body: [
      'Your scale is honest but noisy. One salty dinner, a hard workout, or a short night can move it a kilogram or more — and almost none of that is fat.',
      'So the app doesn’t react to today’s number. It fits a trend line through your recent weigh-ins and shows you the level of that line plus a weekly rate. The trend is the signal; each daily weigh-in is just a data point.',
      'A weekly rate is the honest answer to “is this working?” — much more useful than comparing two random mornings.',
      {
        tip: 'Weigh yourself at the same time each day — after the bathroom, before food or coffee — and let the trend do the thinking.'
      }
    ],
    underTheHood: [
      'The dashboard uses least-squares linear regression over weigh-ins inside the TDEE lookback window (90 days by default, configurable in Profile). A trend is only reported after at least 7 weight observations (MinWeightTrendDays).',
      'The displayed trend weight is the fitted line evaluated at the newest observation, not a forecast. The weekly rate is the slope × 7. Observations outside the plausible 20–300 kg range are ignored.',
      'The dashboard sparkline’s Trend view draws that same fitted line: each point is the engine’s regression level at that weigh-in — the identical numbers the check-in uses, not a separate client-side smoothing. The curve starts drawing at your 6th weigh-in (a two-point line by the 7th), while the headline trend and target updates still need 7.',
      'Observed TDEE uses both your intake and your weight trend: the trend slope tells the model how much energy your body stored or spent. It is not a single weigh-in — the trend is the signal, and targets only move at check-in once the weight term is reliable.'
    ]
  },
  {
    id: 'weekly-check-in',
    title: 'Weekly check-in',
    icon: '📅',
    body: [
      'A check-in is a scheduled look at your data. It estimates how much energy you actually burn by comparing what you ate with what your weight trend says happened — then nudges your calorie target toward the goal you chose.',
      'Two weigh-in gates keep it honest: you need 3 weigh-ins before you can check in at all, and 7 days of weigh-ins before your targets are allowed to move. Until then your estimate is intake-only and your targets stay exactly where they are.',
      'Targets move gradually. A single check-in can change your daily calories by at most 500 kcal in either direction, and never below 1,200 kcal.',
      'Check-ins are available every 5 days. Skipping is allowed — your check-in is still recorded, your targets stay exactly where they are, and you can try again when you have a clearer picture.',
      {
        tip: 'After a holiday, an illness, or a run of missed logs, skipping a check-in is often the smartest move.'
      }
    ],
    underTheHood: [
      'Your estimated burn = what you ate − what your weight trend says your body gained or lost (≈7700 kcal per kg). Needs 7 days of weigh-ins to trust the weight term. Until then your estimate is intake-only and targets won’t move.',
      'In practice the weight term is the slope of your trend line in kg per day, multiplied by 7,700 kcal per kg. It only counts once there are at least 7 distinct weigh-in days spanning 7 calendar days inside the lookback window; short of that the body-energy side is ignored and the estimate leans on intake alone.',
      {
        list: [
          '3 weigh-ins (MinWeighInsForCheckin) unlock the check-in itself. Grip, sleep, and BOLT do not count toward this gate — only entries with a real weight do.',
          '7 days of weigh-ins make the weight term trustworthy. Until then a check-in is still recorded, but the recommendation equals your current target with a zero adjustment — no calorie or macro change.',
          'A check-in can run every 5 days (MinCheckinIntervalDays). That cooldown is separate from the data gates.'
        ]
      },
      'When a target update is allowed, recommendation = current calories + clamp(goal-adjusted observed TDEE − current calories, −500, +500), then floored at 1,200 kcal. The full macro set is recomputed server-side from the final calorie figure, so protein, carbs, fat, fiber, and water always match the stored calories.',
      'The first 14 days after your first weigh-in are a calibration shield. Targets still move, but by no more than ±150 kcal per check-in while the engine verifies that the trend it measured is real rather than water-weight noise. The badge counts the window for you: “Calibrating · day 3 of 14”.',
      'That follows the calibration window used in the MacroFactor / Stronger by Science tradition of adaptive TDEE work, which runs 14–30 days of consistent logging before the energy-balance model is trusted with large changes. We deliberately take the short end: two weeks settles the early noise without the opposite failure, where targets never move and people stop logging.',
      'The shield is not a cage. If the unclamped ideal target is more than 1,200 kcal above your current target — a genuine severe-undereating case — the normal weekly swing is restored for that raise only. A noisy high estimate can never widen a decrease; the backstop only ever lets a real deficit escape the small-step cap.',
      'Skip sends a zero adjustment: the check-in is recorded and targets are left untouched. Either way, a target snapshot is written for the timeline.'
    ]
  },
  {
    id: 'readiness',
    title: 'Readiness',
    icon: '🚦',
    body: [
      'Grip strength and BOLT are your daily vital signs. They’re quick, cheap, and surprisingly sensitive to sleep, stress, and accumulated training.',
      'The app never compares you to anyone else. It compares today with your own recent baseline, so “good” means good for you.',
      'Small dips are normal — one bad night shouldn’t derail a week. A single low reading is information, not a verdict.',
      {
        terms: [
          { term: 'Green', def: 'At or near your baseline. Train as planned.' },
          { term: 'Yellow', def: 'Below baseline for a few days. Consider an easier session or extra recovery.' },
          { term: 'Red', def: 'A longer run below baseline. Back off, prioritize sleep and food, and deload if you’re lifting.' }
        ]
      },
      {
        tip: 'Readiness is a guide, not a command. If you feel great on a yellow day, use your judgment — and if you feel awful on a green day, listen to that too.'
      }
    ],
    underTheHood: [
      'Each signal uses its most recent 14 positive readings. The app computes an EMA with α = 0.3 (ReadinessEMAAlpha), a population standard deviation, and a z-score: (today − EMA) / SD.',
      'Grip and BOLT are combined as a weighted average. The default is 50/50, adjustable in Profile. If only one signal has data, the available signal’s weight is renormalized to 1.0.',
      'Escalation is by consecutive days, not a single snapshot:',
      {
        list: [
          '|z| between 1 and 2 → yellow at 4 consecutive days, red at 5.',
          '|z| of 2 or more → yellow at 2 consecutive days, red at 3.',
          'A day with no biometric entry breaks the streak, because the app cannot confirm a deviation without data.'
        ]
      },
      'Velocity compares the current 7-day average Rz with the previous 7 days. A grip/BOLT mismatch can also trigger yellow: grip high + BOLT low is “Stress Overload”; grip low + BOLT high is “Recovery Hangover”; both low is “Systemic Fatigue.”'
    ]
  },
  {
    id: 'bolt',
    title: 'BOLT score',
    icon: '🫁',
    body: [
      'BOLT stands for Body Oxygen Level Test. It’s the number of seconds you can hold your breath after a normal exhale through your nose.',
      'It is not a maximum breath-hold. Sit calmly, breathe normally, exhale through your nose, pinch your nose, and start counting. Stop at the first real urge to breathe — not when you’re desperate.',
      'A rough way to read the number:',
      {
        terms: [
          { term: 'Under 20 s', def: 'Breathing dysfunction — worth working on.' },
          { term: '20–30 s', def: 'Average.' },
          { term: '30–40 s', def: 'Good.' },
          { term: '40+ s', def: 'Excellent.' }
        ]
      },
      'BOLT reflects breathing efficiency and CO₂ tolerance, which tend to track how well your body handles everyday stress. Log it in the morning, before coffee, for the cleanest comparison.',
      {
        tip: 'Never force it. Dizziness or panic means stop. The score only matters if you measure the same way each time.'
      }
    ],
    underTheHood: [
      'BOLT is logged as boltScore (seconds) in the daily check-in. Only values greater than zero are used.',
      'Readiness takes the most recent 14 BOLT readings and compares today’s value with their EMA (α = 0.3) and standard deviation. If BOLT is missing, readiness leans on grip alone and renormalizes the weights.'
    ]
  },
  {
    id: 'grip-strength',
    title: 'Grip strength',
    icon: '✊',
    body: [
      'Grip strength is measured with a hand dynamometer and logged in kilograms.',
      'Measure at the same time each day, and take the best of 2–3 tries. Pick one hand and keep using it — consistency matters more than which hand you choose.',
      'Why it matters: grip is a cheap window into your nervous system. When you’re run down, sleep-deprived, or carrying fatigue, grip usually drops before you feel it in the gym.',
      {
        tip: 'Warm up briefly and don’t squeeze until your hand aches. A clean, repeatable measurement beats a heroic one.'
      }
    ],
    underTheHood: [
      'Grip is logged as gripKg. Readiness takes the last 14 positive grip values, computes EMA (α = 0.3), standard deviation, and a z-score against today.',
      'Profile gripWeight sets grip’s share of the combined score; the default is 0.5, an even split with BOLT. The engine falls back to 0.5 if the configured share is 0 or 1. The dashboard also tracks your personal best.'
    ]
  },
  {
    id: 'macros',
    title: 'Macros & distribution',
    icon: '🍽️',
    body: [
      'Calories decide your weight. Macros decide what those calories are made of — and how you feel training on them.',
      {
        terms: [
          { term: 'Protein', def: 'Repairs and preserves muscle. It’s the macro worth protecting first when calories get low.' },
          { term: 'Carbs', def: 'Your fastest fuel for hard training and the main way you refill muscle glycogen.' },
          { term: 'Fat', def: 'Hormones, cell membranes, and satiety. There’s a floor for a reason.' }
        ]
      },
      'Distribution styles (set in Profile → Macro Distribution):',
      {
        list: [
          'Keto — carbs capped around 30 g per day; fat takes the rest.',
          'Low carb — carbs capped around 100 g per day; fat takes the rest.',
          'Balanced — 40% of the calories left after protein come from fat; carbs get the remainder.',
          'High carb — 25% of the calories left after protein come from fat; carbs get the remainder.'
        ]
      },
      'Prioritize carbs (for runners): if you run 24 km per week or more, protein is set a little lower (1.6 g/kg, or 1.8 g/kg in a deficit) and carbs get a floor of 5 g/kg.',
      'Eat-back exercise: when enabled, calories burned in workouts are added back to that day’s target, so a hard session doesn’t quietly eat into your deficit.',
      'Saved meals and templates remember the description too — applying one stacks its description into the notes field with newlines, so nothing you typed is lost.',
      'Share meal templates with your group so everyone can apply them.',
      {
        tip: 'The priority order is protein first, then fat to its floor, then carbs fill the rest. That’s exactly how the app handles low calories.'
      }
    ],
    underTheHood: [
      'Protein: 1.5 g/kg base; lifters 1.9 g/kg (1.6 if body fat is above 25%, 2.2 if below 12%), plus 0.3 g/kg in a deficit. High-volume runners who prioritize carbs use 1.6 g/kg (1.8 in a deficit).',
      'Fat floor: 30 g + 0.5 g per cm of height above 150 cm. Fiber: 14 g per 1,000 kcal, floored at 36 g for men and 28 g for women. Water: 35 ml/kg.',
      'After protein calories are subtracted, keto caps carbs at 30 g and low_carb at 100 g; fat takes the remaining energy but never drops below the fat floor (the floor wins and carbs are trimmed). Balanced gives fat 40% of the post-protein remainder; high_carb gives it 25%. Carbs are whatever energy is left.',
      'Goal multipliers: cut 10/20/30% → 0.90/0.80/0.70; maintenance → 1.0; bulk 10/20% → 1.10/1.20. Katch-McArdle replaces Mifflin-St Jeor for BMR when a body-fat percentage is known: 370 + 21.6 × lean mass (kg).'
    ]
  },
  {
    id: 'groups-sharing',
    title: 'Groups & sharing',
    icon: '👥',
    body: [
      'A group is a small circle — family, a training crew, a few friends — who want to see each other keep going without turning it into a leaderboard.',
      'Invites are single-use and expire after 7 days. Generate a code from the group page and send it directly to the person you want to join.',
      'Member cards show only what each person has chosen to share: today’s logging presence, logging streaks, and 7-day adherence. Each member sets their own “My sharing” toggles, per group, and nothing is on by default.',
      'Tap ＋ New in Templates to bundle saved meals into a template, then share it with a group so everyone can apply the same meals.',
      {
        tip: 'Body data — weight, body fat, measurements — is never shareable, by design. A group only ever sees the signals each member explicitly turns on.'
      }
    ],
    underTheHood: [
      'Invite codes are single-use, 10 characters, and expire 7 days after creation. Joining needs a valid unused code and is rate limited server-side.',
      'The group summary is built per viewer: a member’s clock, streak, and adherence appear only when that member has the matching sharing toggle on. Unshared signals are omitted from the response, not hidden in the client.',
      'Shared templates are visible to every group member and carry a “shared by” badge for the owner. The template owner, or the group owner, can unshare them at any time.',
      'There is no body-data field in the group API. Weight, body fat, measurements, and detailed nutrition are never sent to group members.'
    ]
  },
  {
    id: 'workouts',
    title: 'Workouts',
    icon: '🏋️',
    body: [
      'A workout is a session on a date, filed under a slot. The slot is just a label — AM and PM, or 1 and 2 — so two sessions on the same day don’t overwrite each other.',
      'RPE is your rate of perceived exertion, from 1 to 10. Think of 1 as barely moving, 5 as working but comfortable, 7 as hard but sustainable, 9 as one rep left, and 10 as everything you have. Be honest; it’s a better input than a guess.',
      'Leave calories at 0 and the app estimates the burn from METs and your recent bodyweight. If you know the number from a watch, enter it and the app will use yours.',
      'You can type a session, paste YAML for a preplanned workout, or describe it in plain language and let the AI parse it.',
      'Implausible numbers from the model are auto-clamped, and anything uncertain is flagged in the meal notes for your review.',
      {
        tip: 'Always review a parsed session before saving. AI is a fast typist, not a coach. And log the session you actually did, not the one you planned.'
      }
    ],
    underTheHood: [
      'Session identity is date + slot. Saving is an upsert, so re-saving the same date/slot updates that session.',
      'Calories: MET × bodyweight (kg) × duration (hours), using the average of the session’s per-exercise MET values (fallback 4.0 METs if none are supplied). On save with calories = 0, the server estimates from your most recent weight. METs can come from an explicit value, RPE (9–10 → 8, 7–8 → 6, 5–6 → 5, below 5 → 4), or an exercise-name lookup.',
      'Each saved session keeps its date, slot, title, duration, exercises, RPE, and calorie estimate, so your workout history is a plain record you can browse and export.'
    ]
  },
  {
    id: 'body-fat',
    title: 'Body fat',
    icon: '📏',
    body: [
      'The Navy method estimates body fat from tape measurements: neck and waist for men; neck, waist, and hips for women. Height comes from your profile.',
      'Measure in the same place each time, with the tape snug but not compressing the skin. The Measurements page stores the numbers; once neck and waist (and hips, if needed) are in, the app offers a one-tap “Auto-fill body fat” back into your biometrics.',
      'Why it matters: body fat makes the calorie math leaner. When the app knows your body fat, it can estimate your resting burn from lean mass instead of just weight, height, age, and sex.',
      {
        tip: 'Don’t chase a single decimal place. Body-fat estimates are directional; the trend over weeks is what counts.'
      }
    ],
    underTheHood: [
      'Navy formulas: men use log10(waist − neck) and log10(height); women use log10(waist + hips − neck) and log10(height). Results outside 2–80% are treated as invalid (0).',
      'The Body Fat page prefills its calculator from your latest usable measurement row, saves the measurements first, then asks the server to calculate. The Measurements page can write the Navy result back to that day’s biometrics without touching your other biometric fields.',
      'When profile body fat is above 0, BMR uses Katch-McArdle: 370 + 21.6 × lean mass (kg), where lean mass = weight × (1 − body-fat % / 100). Otherwise it falls back to Mifflin-St Jeor.'
    ]
  },
  {
    id: 'glossary',
    title: 'Glossary',
    icon: '📖',
    body: [
      'The short version of every term that shows up in the app.',
      {
        terms: [
          { term: 'BOLT', def: 'Body Oxygen Level Test — seconds you can hold your breath after a normal nose exhale, stopping at the first real urge to breathe.' },
          { term: 'EMA', def: 'Exponential moving average — a smoothed average that gives recent values more weight.' },
          { term: 'MET', def: 'Metabolic equivalent of task — how many times resting energy use an activity costs (1 MET is roughly resting).' },
          { term: 'Observed TDEE', def: 'Your estimated daily energy burn, from what you ate minus what your weight trend says you gained or lost.' },
          { term: 'RPE', def: 'Rate of perceived exertion, from 1 (barely moving) to 10 (everything you have).' },
          { term: 'Readiness', def: 'A green/yellow/red signal comparing today’s grip and BOLT with your own recent baseline.' },
          { term: 'Trend weight', def: 'The fitted line through your recent weigh-ins, used instead of a single scale reading.' },
          { term: 'Eat-back', def: 'Adding workout calories back to that day’s food target.' },
          { term: 'Distribution', def: 'How the calories left after protein are split between fat and carbs.' },
          { term: 'Check-in', def: 'A periodic target recalculation, at least 5 days apart. Needs 3 weigh-ins to run and 7 days of weigh-ins before targets can move.' },
          { term: 'Slot', def: 'The label that separates multiple sessions on the same date, such as AM/PM or 1/2.' },
          { term: 'Workout history', def: 'Your saved sessions by date and slot, each with duration, exercises, RPE, and the calorie estimate the app computed for it.' }
        ]
      }
    ]
  }
]
