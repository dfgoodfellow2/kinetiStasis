import { api } from './api.js'

// Ask the server to distribute a calorie target across macros using the
// macroDistribution stored on the user's profile. Returns whole-number values
// ready for display/binding.
//
// The server response includes `fiberG` and `calories`, so keep the full shape
// instead of narrowing to the four macros shown in the UI.
export async function autoDistribute(calories) {
  const cal = Number(calories)
  const result = await api.distributeMacros(cal)

  return {
    calories: Math.round(result.calories ?? cal),
    proteinG: Math.round(result.proteinG ?? 0),
    carbsG: Math.round(result.carbsG ?? 0),
    fatG: Math.round(result.fatG ?? 0),
    fiberG: Math.round(result.fiberG ?? 0),
    waterMl: Math.round(result.waterMl ?? 0),
  }
}
