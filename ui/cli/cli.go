package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const helpText = `dietctl — read + whitelisted-write client for Diet Tracker

USAGE
  dietctl [--json] <command> [flags]

GLOBAL OPTIONS
  --json              Print the raw response JSON (intended for AI/tool use).
  -h, --help          Show this help.

ENVIRONMENT
  DIET_API_URL        API base URL. Default: https://diet-tracker-v2.fly.dev
  DIET_LOGIN          Login (username or email) for "dietctl login".
  DIET_PASSWORD       Password for "dietctl login".

SESSION
  Cookies are stored in ~/.config/diet-tracker-v2/dietctl-cookies.json
  (directory 0700, file 0600). The refresh cookie is scoped to /v1/auth.
  On a 401 the CLI refreshes once and retries the original request once.

AUTH COMMANDS
  login [--login X] [--password Y]
        Log in and persist the session cookies. Missing values fall back to
        DIET_LOGIN / DIET_PASSWORD, then to interactive prompts.
  logout
        Revoke the server session and delete the local cookie file.
  whoami
        GET /v1/auth/me

READ COMMANDS (dates are YYYY-MM-DD, default today UTC)
  nutrition [--from --to]          GET /v1/nutrition/logs
  biometrics [--from --to]         GET /v1/biometrics
  workouts [--from --to]           GET /v1/workouts
  targets                          GET /v1/targets
  targets-history                  GET /v1/targets/history
  measurements [--from --to]       GET /v1/measurements
  checkin-preview                  GET /v1/checkin
  checkin-history                  GET /v1/checkin/history
  dashboard [--date]               GET /v1/dashboard
        The richest single call: today's totals, adaptive TDEE, readiness,
        weekly stats, weight trend and check-in state.
  export <combined|nutrition|workouts> [--from --to] [--format md|csv]
        GET /v1/export/<kind>. Default range is today only; default format md.
        Human output prints the export body; --json requests the JSON wrapper.

WRITE COMMANDS (whitelist)
  Every write is restricted to TODAY'S UTC DATE. Supplying --date with any
  other value is rejected. This is enforced here, in the CLI: POST upserts can
  be last-write-wins, so arbitrary dates could rewrite history.

  workout-add --file workout.yaml [--date YYYY-MM-DD]
        Parse YAML through POST /v1/parse/workout, print the mapped JSON
        summary to stderr, then POST /v1/workouts. Parser errors exit 1.
        Slot comes from the parsed workout, otherwise "1".

  biometric-add [--grip-kg X] [--bolt-score N] [--weight-kg X]
                [--waist-cm X] [--sleep-hours X] [--notes TEXT]
        POST /v1/biometrics with only the provided non-zero fields; the server
        merges them into today's row. At least one field is required.

  nutrition-add [--kcal N] [--protein-g N] [--carbs-g N] [--fat-g N]
                [--fiber-g N] [--water-ml N] [--note TEXT]
        POST /v1/nutrition/logs for today. Nutrition values are additive on
        the server.

  measurement-add [--neck-cm X] [--chest-cm X] [--waist-cm X] [--hips-cm X]
                  [--thigh-cm X] [--bicep-cm X] [--shoulders-cm X]
                  [--calves-cm X] [--note TEXT]
        POST /v1/measurements for today.

  grip-weight <0.0-1.0>
        The bolt/grip readiness setting. Reads the complete current profile,
        changes ONLY gripWeight, and writes the full profile back. This is the
        only command that performs a full-row replacement.

  checkin [--weight-before X] [--weight-after X] [--calories-before N]
          [--calories-after N] [--reason TEXT]
        POST /v1/checkin for today. A 409 means today's check-in already
        exists and is reported without retrying.

        WARNING: if --calories-after is greater than zero, the server also
        updates your calorie targets as part of the check-in.

SAFETY MODEL
  - There are no update, remove, or partial-update commands.
  - The only non-GET, non-POST call is grip-weight's full profile write.
  - The remaining write commands accept --date only so a non-today value can
    be rejected; no request is made when the date is not today UTC.
  - All request bodies use exact API field names; no extra keys are sent.

EXAMPLES
  dietctl login --login coach@example.com
  dietctl dashboard --json
  dietctl nutrition --from 2026-10-01 --to 2026-10-31
  dietctl nutrition-add --kcal 450 --protein-g 35 --note "chicken bowl"
  dietctl workout-add --file workout.yaml
  dietctl grip-weight 0.7
  dietctl export combined --from 2026-10-01 --to 2026-10-31 --format md
`

var errHelp = flag.ErrHelp

// Run executes one CLI invocation and returns a process exit code.
func Run(args []string) int {
	args, jsonOut := extractJSON(args)

	if len(args) == 0 {
		fmt.Print(helpText)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help", "-help":
		fmt.Print(helpText)
		return 0
	}

	client, err := NewClient()
	if err != nil {
		fail(err)
		return 1
	}

	command := args[0]
	rest := args[1:]

	switch command {
	case "login":
		err = cmdLogin(client, rest, jsonOut)
	case "logout":
		err = cmdLogout(client, rest, jsonOut)
	case "whoami":
		err = cmdSimpleGet(client, "whoami", "/v1/auth/me", rest, jsonOut)
	case "nutrition":
		err = cmdRangeGet(client, "nutrition", "/v1/nutrition/logs", rest, jsonOut)
	case "biometrics":
		err = cmdRangeGet(client, "biometrics", "/v1/biometrics", rest, jsonOut)
	case "workouts":
		err = cmdRangeGet(client, "workouts", "/v1/workouts", rest, jsonOut)
	case "targets":
		err = cmdSimpleGet(client, "targets", "/v1/targets", rest, jsonOut)
	case "targets-history":
		err = cmdSimpleGet(client, "targets-history", "/v1/targets/history", rest, jsonOut)
	case "measurements":
		err = cmdRangeGet(client, "measurements", "/v1/measurements", rest, jsonOut)
	case "checkin-preview":
		err = cmdSimpleGet(client, "checkin-preview", "/v1/checkin", rest, jsonOut)
	case "checkin-history":
		err = cmdSimpleGet(client, "checkin-history", "/v1/checkin/history", rest, jsonOut)
	case "dashboard":
		err = cmdDashboard(client, rest, jsonOut)
	case "export":
		err = cmdExport(client, rest, jsonOut)
	case "workout-add":
		err = cmdWorkoutAdd(client, rest, jsonOut)
	case "biometric-add":
		err = cmdBiometricAdd(client, rest, jsonOut)
	case "nutrition-add":
		err = cmdNutritionAdd(client, rest, jsonOut)
	case "measurement-add":
		err = cmdMeasurementAdd(client, rest, jsonOut)
	case "grip-weight":
		err = cmdGripWeight(client, rest, jsonOut)
	case "checkin":
		err = cmdCheckin(client, rest, jsonOut)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\nRun `dietctl --help` to see the available commands.\n", command)
		return 2
	}

	if err == nil {
		return 0
	}
	if errors.Is(err, errHelp) {
		return 0
	}
	fail(err)
	return 1
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
}

// extractJSON removes standalone --json/-json flags from anywhere in argv so
// they work before or after the command.
func extractJSON(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json", "-json":
			jsonOut = true
		default:
			out = append(out, arg)
		}
	}
	return out, jsonOut
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Print(helpText)
	}
	return fs
}

func parseArgs(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q; run `dietctl --help`", strings.Join(fs.Args(), " "))
	}
	return nil
}

func todayUTC() string {
	return time.Now().UTC().Format("2006-01-02")
}

func validateDate(value string) error {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return fmt.Errorf("date must be in YYYY-MM-DD format, got %q", value)
	}
	return nil
}

// resolveWriteDate enforces the write whitelist: the target date must be today
// in UTC. Any other date is refused before a request is made.
func resolveWriteDate(requested string) (string, error) {
	today := todayUTC()
	if requested == "" {
		return today, nil
	}
	if err := validateDate(requested); err != nil {
		return "", err
	}
	if requested != today {
		return "", fmt.Errorf("refusing write: %s is not today (%s UTC); writes are restricted to the current UTC day", requested, today)
	}
	return today, nil
}
