package units

const (
	LbsToKg  = 0.453592
	KgToLbs  = 2.20462
	CmToInch = 0.393701
	InchToCm = 2.54
	KmToMi   = 0.621371
	MiToKm   = 1.60934
	MToFt    = 3.28084
	FtToM    = 0.3048
)

func LbsToKilograms(lbs float64) float64 { return lbs * LbsToKg }
func KilogramsToLbs(kg float64) float64  { return kg * KgToLbs }
func CmToInches(cm float64) float64      { return cm * CmToInch }
func InchesToCm(in float64) float64      { return in * InchToCm }

func KilometersToMiles(km float64) float64 { return km * KmToMi }
func MilesToKilometers(mi float64) float64 { return mi * MiToKm }

func MetersToFeet(m float64) float64  { return m * MToFt }
func FeetToMeters(ft float64) float64 { return ft * FtToM }
