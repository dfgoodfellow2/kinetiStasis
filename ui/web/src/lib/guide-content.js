// Plain-language guide content for the in-app manual.
//
// Every number in this file is sourced from the Go implementation so the
// guide cannot drift away from what the app actually does:
//   - internal/constants/constants.go (gating, floors, readiness alpha)
//   - internal/services/readiness/readiness.go (z-scores, escalation zones)
//   - internal/services/nutrition/macros.go (protein/fat/carb/fiber/water)
//   - internal/services/calculator/adaptive.go (tiering, damping, trend)
//   - internal/services/met/met.go + exercise/volume.go (MET, MWV, NDS)
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
      }
    ],
    underTheHood: [
      'Daily nutrition logs feed the observed-TDEE estimate. Biometric logs feed the weight trend and readiness.',
      {
        list: [
          'Check-ins unlock after at least 5 days (MinCheckinIntervalDays) and need at least 3 biometric log days (MinBiometricLogsForCheckin).',
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
      'Observed TDEE itself is estimated from your logged intake history, not from a single weigh-in. The weight trend is shown alongside it for context, and targets only move at check-in.'
    ]
  },
  {
    id: 'weekly-check-in',
    title: 'Weekly check-in',
    icon: '📅',
    body: [
      'A check-in is a scheduled look at your data. It recalculates your observed TDEE — your estimated daily burn, recalculated from your own logged intake — and nudges your calorie target toward the goal you chose.',
      'Targets move gradually. A single check-in can change your daily calories by at most 500 kcal in either direction, and never below 1,200 kcal.',
      'Check-ins unlock every 5 days and need at least 3 days of biometric data. That’s deliberate: it keeps one weird Tuesday from rewriting the whole plan.',
      'Skipping is allowed. Your check-in is still recorded, your targets stay exactly where they are, and you can try again when you have a clearer picture.',
      {
        tip: 'After a holiday, an illness, or a run of missed logs, skipping a check-in is often the smartest move.'
      }
    ],
    underTheHood: [
      'Observed TDEE is tiered by how much intake data you have in the lookback window (90 days by default; when no lookback is set, high-volume runners above 32 km/week use 28 days):',
      {
        list: [
          'Under 7 days: linear regression. Confidence becomes medium at 4+ days.',
          '7–29 days: recursive least squares with a forgetting factor of 0.98. Confidence is medium below 14 days, high from 14.',
          '30+ days: exponential moving average with α = 0.10. Confidence high.'
        ]
      },
      'Recommendation = current calories + clamp(goal-adjusted observed TDEE − current calories, −500, +500), then floored at 1,200 kcal. The ±500 cap is MaxWeeklyCalorieSwing.',
      'The preview is gated by canCheckIn: at least 5 days since the last check-in AND at least 3 biometric logs. Accept writes the recommended calories; Skip sends 0 and leaves targets untouched. Either way, a target snapshot is recorded for the timeline.'
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
    id: 'workouts',
    title: 'Workouts',
    icon: '🏋️',
    body: [
      'A workout is a session on a date, filed under a slot. The slot is just a label — AM and PM, or 1 and 2 — so two sessions on the same day don’t overwrite each other.',
      'RPE is your rate of perceived exertion, from 1 to 10. Think of 1 as barely moving, 5 as working but comfortable, 7 as hard but sustainable, 9 as one rep left, and 10 as everything you have. Be honest; it’s a better input than a guess.',
      'Leave calories at 0 and the app estimates the burn from METs and your recent bodyweight. If you know the number from a watch, enter it and the app will use yours.',
      'You can type a session, paste YAML for a preplanned workout, or describe it in plain language and let the AI parse it.',
      {
        tip: 'Always review a parsed session before saving. AI is a fast typist, not a coach. And log the session you actually did, not the one you planned.'
      }
    ],
    underTheHood: [
      'Session identity is date + slot. Saving is an upsert, so re-saving the same date/slot updates that session.',
      'Calories: MET × bodyweight (kg) × duration (hours), using the average of the session’s per-exercise MET values (fallback 4.0 METs if none are supplied). On save with calories = 0, the server estimates from your most recent weight. METs can come from an explicit value, RPE (9–10 → 8, 7–8 → 6, 5–6 → 5, below 5 → 4), or an exercise-name lookup.',
      'Training load stored on each session: MWV is the sum of load (lb) × reps, excluding warmup and cooldown; NDS (Neural Demand Score) weights volume by joint involvement, unilateral bias, and intensity; session density is MET-minutes divided by clock minutes. All three are included in exports.'
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
          { term: 'Observed TDEE', def: 'Your estimated daily energy burn, recalculated from your own logged data.' },
          { term: 'RPE', def: 'Rate of perceived exertion, from 1 (barely moving) to 10 (everything you have).' },
          { term: 'Readiness', def: 'A green/yellow/red signal comparing today’s grip and BOLT with your own recent baseline.' },
          { term: 'Trend weight', def: 'The fitted line through your recent weigh-ins, used instead of a single scale reading.' },
          { term: 'Eat-back', def: 'Adding workout calories back to that day’s food target.' },
          { term: 'Distribution', def: 'How the calories left after protein are split between fat and carbs.' },
          { term: 'Check-in', def: 'A periodic target recalculation, at least 5 days apart and requiring at least 3 biometric days.' },
          { term: 'Slot', def: 'The label that separates multiple sessions on the same date, such as AM/PM or 1/2.' },
          { term: 'MWV', def: 'Mechanical work volume — load × reps summed across a session’s working sets, excluding warmup and cooldown.' },
          { term: 'NDS', def: 'Neural Demand Score — a training-load number that weights volume by joint involvement, unilateral bias, and intensity.' },
          { term: 'Session density', def: 'MET-minutes divided by the session’s clock minutes — how much work you packed into the time.' }
        ]
      }
    ]
  }
]
