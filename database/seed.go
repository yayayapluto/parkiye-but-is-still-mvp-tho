package database

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	auditDomain "parkieee/internal/modules/audit"
	authDomain "parkieee/internal/modules/auth"
	feeDomain "parkieee/internal/modules/fee"
	ocrDomain "parkieee/internal/modules/ocr"
	overrideDomain "parkieee/internal/modules/override"
	paymentDomain "parkieee/internal/modules/payment"
	rfidDomain "parkieee/internal/modules/rfid"
	txDomain "parkieee/internal/modules/transaction"
	vehicleDomain "parkieee/internal/modules/vehicle"
	zoneDomain "parkieee/internal/modules/zone"
	"parkieee/pkg/types"
)

// Seed inserts base data required for the app to function, then fills each
// module with realistic random rows (25–80 per entity where applicable).
// Idempotent: all inserts use ON CONFLICT DO NOTHING.
func Seed(db *gorm.DB) error {
	gofakeit.Seed(time.Now().UnixNano())

	steps := []struct {
		name string
		fn   func(*gorm.DB) error
	}{
		{"roles", seedRoles},
		{"permissions", seedPermissions},
		{"role_permissions", seedRolePermissions},
		{"admin user", seedAdminUser},
		{"users", seedUsers},
		{"vehicle_types", seedVehicleTypes},
		{"vehicles", seedVehicles},
		{"zones + gates", seedZonesAndGates},
		{"rfid_cards", seedRFIDCards},
		{"fee_configs + tiers", seedFeeConfigsAndTiers},
		{"holiday_rates", seedHolidayRates},
		{"ocr_config", seedOCRConfig},
		{"override_config", seedOverrideConfig},
		{"transactions", seedTransactions},
		{"payments", seedPayments},
		{"operator_overrides", seedOperatorOverrides},
		{"ocr_jobs + results", seedOCRJobsAndResults},
		{"audit_logs", seedAuditLogs},
	}

	for _, s := range steps {
		if err := s.fn(db); err != nil {
			return fmt.Errorf("seed %s: %w", s.name, err)
		}
	}
	return nil
}

// ── auth ─────────────────────────────────────────────────────────────────────

func seedRoles(db *gorm.DB) error {
	roles := []authDomain.Role{
		{ID: roleID("operator"), Name: string(types.RoleOperator), Description: "Gate operator — handles entry/exit and overrides"},
		{ID: roleID("admin"), Name: string(types.RoleAdmin), Description: "Administrator — manages users, zones, and fee configs"},
		{ID: roleID("owner"), Name: string(types.RoleOwner), Description: "Business owner — full read access + holiday rate management"},
		{ID: roleID("engineer"), Name: string(types.RoleEngineer), Description: "Engineer — audit log access and system configuration"},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&roles).Error
}

func seedPermissions(db *gorm.DB) error {
	perms := []authDomain.Permission{
		{ID: permID("gate.override"), Node: string(types.PermGateOverride), Description: "Manually open gate or override exit"},
		{ID: permID("fee.edit"), Node: string(types.PermFeeEdit), Description: "Create and modify fee configurations and tiers"},
		{ID: permID("report.view"), Node: string(types.PermReportView), Description: "View revenue reports and transaction summaries"},
		{ID: permID("user.manage"), Node: string(types.PermUserManage), Description: "Create, deactivate, and assign roles to users"},
		{ID: permID("zone.manage"), Node: string(types.PermZoneManage), Description: "Create and configure zones and gates"},
		{ID: permID("rfid.manage"), Node: string(types.PermRFIDManage), Description: "Deactivate RFID cards and manage card registry"},
		{ID: permID("audit.read"), Node: string(types.PermAuditRead), Description: "Read audit logs and export audit data"},
		{ID: permID("config.edit"), Node: string(types.PermConfigEdit), Description: "Edit system configs (OCR threshold, override limits)"},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&perms).Error
}

func seedRolePermissions(db *gorm.DB) error {
	matrix := map[string][]string{
		"operator": {"gate.override"},
		"admin":    {"gate.override", "fee.edit", "report.view", "user.manage", "zone.manage", "rfid.manage", "config.edit"},
		"owner":    {"report.view", "fee.edit", "zone.manage", "config.edit"},
		"engineer": {"audit.read", "config.edit"},
	}

	now := time.Now()
	var rps []authDomain.RolePermission
	for roleName, nodes := range matrix {
		for _, node := range nodes {
			rps = append(rps, authDomain.RolePermission{
				ID:           deterministicUUID(roleName + ":" + node),
				RoleID:       roleID(roleName),
				PermissionID: permID(node),
				GrantedAt:    now,
			})
		}
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rps).Error
}

func seedAdminUser(db *gorm.DB) error {
	var count int64
	db.Model(&authDomain.User{}).Where("email = ?", "admin@parkieee.local").Count(&count)
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("Admin@123!"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return db.Create(&authDomain.User{
		ID:           deterministicUUID("seed:admin"),
		Name:         "System Admin",
		Email:        "admin@parkieee.local",
		PasswordHash: string(hash),
		RoleID:       roleID("admin"),
		IsActive:     true,
	}).Error
}

func seedUsers(db *gorm.DB) error {
	var existing int64
	db.Model(&authDomain.User{}).Where("email != ?", "admin@parkieee.local").Count(&existing)
	if existing > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("Parkieee@123!"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	roleNames := []string{"operator", "operator", "operator", "owner", "engineer"}
	n := randBetween(25, 80)
	users := make([]authDomain.User, 0, n)
	seenEmails := map[string]bool{"admin@parkieee.local": true}

	for i := 0; i < n; i++ {
		email := uniqueEmail(seenEmails)
		seenEmails[email] = true
		users = append(users, authDomain.User{
			ID:           uuid.New(),
			Name:         gofakeit.Name(),
			Email:        email,
			PasswordHash: string(hash),
			RoleID:       roleID(roleNames[rand.Intn(len(roleNames))]),
			IsActive:     gofakeit.Bool(),
		})
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&users).Error
}

// ── vehicle ───────────────────────────────────────────────────────────────────

func seedVehicleTypes(db *gorm.DB) error {
	vts := []vehicleDomain.VehicleType{
		{ID: deterministicUUID("vtype:motorcycle"), Name: "motorcycle", MinimumFee: 2000, Description: "Sepeda motor"},
		{ID: deterministicUUID("vtype:car"), Name: "car", MinimumFee: 5000, Description: "Mobil penumpang"},
		{ID: deterministicUUID("vtype:truck"), Name: "truck", MinimumFee: 10000, Description: "Kendaraan berat / truk"},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&vts).Error
}

func seedVehicles(db *gorm.DB) error {
	var existing int64
	db.Model(&vehicleDomain.Vehicle{}).Count(&existing)
	if existing > 0 {
		return nil
	}

	// weighted: 60% motorcycle, 30% car, 10% truck
	vtypePool := make([]uuid.UUID, 0, 10)
	for i := 0; i < 6; i++ {
		vtypePool = append(vtypePool, deterministicUUID("vtype:motorcycle"))
	}
	for i := 0; i < 3; i++ {
		vtypePool = append(vtypePool, deterministicUUID("vtype:car"))
	}
	vtypePool = append(vtypePool, deterministicUUID("vtype:truck"))

	sources := []types.VehicleSource{types.VehicleSourceOCR, types.VehicleSourceManualOverride}

	n := randBetween(25, 80)
	vehicles := make([]vehicleDomain.Vehicle, 0, n)
	seenPlates := map[string]bool{}

	for i := 0; i < n; i++ {
		plate := uniquePlate(seenPlates)
		seenPlates[plate] = true
		vehicles = append(vehicles, vehicleDomain.Vehicle{
			ID:            uuid.New(),
			PlateNumber:   plate,
			VehicleTypeID: vtypePool[rand.Intn(len(vtypePool))],
			Source:        sources[rand.Intn(len(sources))],
			Notes:         gofakeit.RandomString([]string{"", "", gofakeit.Sentence(4)}),
		})
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&vehicles).Error
}

// ── zone + gate ───────────────────────────────────────────────────────────────

func seedZonesAndGates(db *gorm.DB) error {
	var existing int64
	db.Model(&zoneDomain.Zone{}).Count(&existing)
	if existing > 0 {
		// Backfill for_vehicle_type_id on zones that were seeded without it.
		vtMotorcycleFix := deterministicUUID("vtype:motorcycle")
		vtCarFix := deterministicUUID("vtype:car")
		db.Model(&zoneDomain.Zone{}).Where("id = ? AND for_vehicle_type_id IS NULL", deterministicUUID("zone:motor")).
			Update("for_vehicle_type_id", vtMotorcycleFix)
		db.Model(&zoneDomain.Zone{}).Where("id IN ? AND for_vehicle_type_id IS NULL",
			[]uuid.UUID{deterministicUUID("zone:mobil"), deterministicUUID("zone:vip")}).
			Update("for_vehicle_type_id", vtCarFix)
		// Assign motorcycle to remaining null zones as a safe default.
		db.Model(&zoneDomain.Zone{}).Where("for_vehicle_type_id IS NULL").
			Update("for_vehicle_type_id", vtMotorcycleFix)
		return nil
	}

	adminID := deterministicUUID("seed:admin")

	vtMotorcycle := deterministicUUID("vtype:motorcycle")
	vtCar := deterministicUUID("vtype:car")
	vtTruck := deterministicUUID("vtype:truck")
	vtPool := []uuid.UUID{vtMotorcycle, vtMotorcycle, vtMotorcycle, vtCar, vtCar, vtCar, vtTruck}

	fixed := []zoneDomain.Zone{
		{ID: deterministicUUID("zone:motor"), Name: "Parkir Motor", Description: "Area parkir sepeda motor lantai 1", Capacity: 200, AdditionalFee: 0, ForVehicleTypeID: &vtMotorcycle, IsActive: true, CreatedBy: &adminID},
		{ID: deterministicUUID("zone:mobil"), Name: "Parkir Mobil", Description: "Area parkir mobil lantai 2", Capacity: 80, AdditionalFee: 2000, ForVehicleTypeID: &vtCar, IsActive: true, CreatedBy: &adminID},
		{ID: deterministicUUID("zone:vip"), Name: "Parkir VIP", Description: "Area parkir VIP covered basement", Capacity: 20, AdditionalFee: 5000, ForVehicleTypeID: &vtCar, IsActive: true, CreatedBy: &adminID},
	}

	areaWords := []string{"Gedung", "Blok", "Lantai", "Area", "Sektor", "Zona"}
	labels := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	feeOptions := []int{0, 0, 1000, 2000, 3000, 5000}
	seenNames := map[string]bool{"Parkir Motor": true, "Parkir Mobil": true, "Parkir VIP": true}

	extraN := randBetween(25, 80) - len(fixed)
	extra := make([]zoneDomain.Zone, 0, extraN)
	for i := 0; i < extraN; i++ {
		name := uniqueZoneName(seenNames, areaWords, labels)
		seenNames[name] = true
		vtID := vtPool[rand.Intn(len(vtPool))]
		extra = append(extra, zoneDomain.Zone{
			ID:               uuid.New(),
			Name:             name,
			Description:      gofakeit.Sentence(6),
			Capacity:         gofakeit.IntRange(10, 300),
			AdditionalFee:    feeOptions[rand.Intn(len(feeOptions))],
			ForVehicleTypeID: &vtID,
			IsActive:         gofakeit.Bool(),
			CreatedBy:        &adminID,
		})
	}

	allZones := append(fixed, extra...)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&allZones).Error; err != nil {
		return err
	}

	locationDescs := []string{
		"Pintu utara", "Pintu selatan", "Pintu timur", "Pintu barat",
		"Pintu basement", "Pintu utama", "Pintu samping", "Pintu darurat",
	}

	var gates []zoneDomain.Gate
	for _, z := range allZones {
		gates = append(gates,
			zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Masuk", GateType: types.GateTypeEntry, LocationDesc: pick(locationDescs), IsActive: true, CreatedBy: &adminID},
			zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Keluar", GateType: types.GateTypeExit, LocationDesc: pick(locationDescs), IsActive: true, CreatedBy: &adminID},
		)
		if gofakeit.Bool() {
			gates = append(gates,
				zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Masuk 2", GateType: types.GateTypeEntry, LocationDesc: pick(locationDescs), IsActive: gofakeit.Bool(), CreatedBy: &adminID},
				zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Keluar 2", GateType: types.GateTypeExit, LocationDesc: pick(locationDescs), IsActive: gofakeit.Bool(), CreatedBy: &adminID},
			)
		}
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&gates).Error
}

// ── rfid ─────────────────────────────────────────────────────────────────────

func seedRFIDCards(db *gorm.DB) error {
	var existing int64
	db.Model(&rfidDomain.RFIDCard{}).Count(&existing)
	if existing > 0 {
		return nil
	}

	var vehicles []vehicleDomain.Vehicle
	db.Select("id").Find(&vehicles)
	vehicleIDs := make([]uuid.UUID, len(vehicles))
	for i, v := range vehicles {
		vehicleIDs[i] = v.ID
	}

	n := randBetween(25, 80)
	cards := make([]rfidDomain.RFIDCard, 0, n)
	seenUIDs := map[string]bool{}

	for i := 0; i < n; i++ {
		uid := uniqueCardUID(seenUIDs)
		seenUIDs[uid] = true

		card := rfidDomain.RFIDCard{
			ID:        uuid.New(),
			CardUID:   uid,
			IsActive:  gofakeit.Bool(),
			CreatedAt: gofakeit.DateRange(time.Now().AddDate(-1, 0, 0), time.Now()),
		}
		if len(vehicleIDs) > 0 && gofakeit.Bool() {
			vid := vehicleIDs[rand.Intn(len(vehicleIDs))]
			card.VehicleID = &vid
		}
		cards = append(cards, card)
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&cards).Error
}

// ── fee ───────────────────────────────────────────────────────────────────────

func seedFeeConfigsAndTiers(db *gorm.DB) error {
	adminID := deterministicUUID("seed:admin")
	now := time.Now()

	// Deterministic fee configs for the 3 fixed zones x 3 vehicle types.
	// grace_period_minutes=0 and base_fee>0 ensures calculated_fee is never 0
	// even for very short parking durations (important for payment testing).
	type tierDef struct{ dur, fee int }
	type fixedCfg struct {
		zoneKey string
		vtKey   string
		baseFee int
		tiers   []tierDef
	}
	fixed := []fixedCfg{
		{"zone:motor", "vtype:motorcycle", 2000, []tierDef{{60, 2000}, {120, 3000}, {240, 5000}}},
		{"zone:motor", "vtype:car", 3000, []tierDef{{60, 3000}, {120, 5000}, {240, 8000}}},
		{"zone:motor", "vtype:truck", 5000, []tierDef{{60, 5000}, {120, 8000}, {240, 12000}}},
		{"zone:mobil", "vtype:motorcycle", 2000, []tierDef{{60, 2000}, {120, 3000}, {240, 5000}}},
		{"zone:mobil", "vtype:car", 5000, []tierDef{{60, 5000}, {120, 8000}, {240, 12000}}},
		{"zone:mobil", "vtype:truck", 8000, []tierDef{{60, 8000}, {120, 12000}, {240, 20000}}},
		{"zone:vip", "vtype:motorcycle", 3000, []tierDef{{60, 3000}, {120, 5000}, {240, 8000}}},
		{"zone:vip", "vtype:car", 10000, []tierDef{{60, 10000}, {120, 15000}, {240, 25000}}},
		{"zone:vip", "vtype:truck", 15000, []tierDef{{60, 15000}, {120, 20000}, {240, 30000}}},
	}

	var configs []feeDomain.FeeConfig
	var tiers []feeDomain.FeeTier

	for _, f := range fixed {
		cfgID := deterministicUUID("fee:" + f.zoneKey + ":" + f.vtKey)
		configs = append(configs, feeDomain.FeeConfig{
			ID:                 cfgID,
			ZoneID:             deterministicUUID(f.zoneKey),
			VehicleTypeID:      deterministicUUID(f.vtKey),
			BaseFee:            f.baseFee,
			GracePeriodMinutes: 0,
			IsActive:           true,
			EffectiveFrom:      now.AddDate(-1, 0, 0),
			CreatedBy:          &adminID,
		})
		for i, t := range f.tiers {
			tiers = append(tiers, feeDomain.FeeTier{
				ID:              deterministicUUID(fmt.Sprintf("tier:%s:%s:%d", f.zoneKey, f.vtKey, i)),
				FeeConfigID:     cfgID,
				TierOrder:       i + 1,
				DurationMinutes: t.dur,
				FeeAmount:       t.fee,
				IsLastTier:      i == len(f.tiers)-1,
			})
		}
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&configs).Error; err != nil {
		return err
	}

	// Verify which config IDs actually exist in DB before inserting tiers,
	// because ON CONFLICT DO NOTHING silently skips duplicates and tiers
	// would violate the FK if their parent config was skipped.
	cfgIDSet := map[uuid.UUID]bool{}
	for _, c := range configs {
		cfgIDSet[c.ID] = true
	}
	var existingCfgs []feeDomain.FeeConfig
	cfgIDs := make([]uuid.UUID, 0, len(configs))
	for id := range cfgIDSet {
		cfgIDs = append(cfgIDs, id)
	}
	db.Where("id IN ?", cfgIDs).Select("id").Find(&existingCfgs)
	validCfgIDs := map[uuid.UUID]bool{}
	for _, c := range existingCfgs {
		validCfgIDs[c.ID] = true
	}

	var validTiers []feeDomain.FeeTier
	for _, t := range tiers {
		if validCfgIDs[t.FeeConfigID] {
			validTiers = append(validTiers, t)
		}
	}
	if len(validTiers) > 0 {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&validTiers).Error; err != nil {
			return err
		}
	}

	// Random configs for the remaining (non-fixed) zones, skip if already seeded
	var existingCount int64
	db.Model(&feeDomain.FeeConfig{}).Count(&existingCount)
	if existingCount > int64(len(fixed)) {
		return nil
	}

	fixedZoneIDs := map[uuid.UUID]bool{
		deterministicUUID("zone:motor"): true,
		deterministicUUID("zone:mobil"): true,
		deterministicUUID("zone:vip"):   true,
	}
	vtypeIDs := []uuid.UUID{
		deterministicUUID("vtype:motorcycle"),
		deterministicUUID("vtype:car"),
		deterministicUUID("vtype:truck"),
	}

	var allZones []zoneDomain.Zone
	db.Select("id").Find(&allZones)

	var randConfigs []feeDomain.FeeConfig
	var randTiers []feeDomain.FeeTier

	for _, z := range allZones {
		if fixedZoneIDs[z.ID] {
			continue
		}
		for _, vtID := range vtypeIDs {
			cfgID := uuid.New()
			randConfigs = append(randConfigs, feeDomain.FeeConfig{
				ID:                 cfgID,
				ZoneID:             z.ID,
				VehicleTypeID:      vtID,
				BaseFee:            pick([]int{1000, 2000, 3000}),
				GracePeriodMinutes: pick([]int{0, 5, 10}),
				IsActive:           true,
				EffectiveFrom:      now.AddDate(-1, 0, 0),
				CreatedBy:          &adminID,
			})
			tierCount := gofakeit.IntRange(2, 3)
			for t := 1; t <= tierCount; t++ {
				randTiers = append(randTiers, feeDomain.FeeTier{
					ID:              uuid.New(),
					FeeConfigID:     cfgID,
					TierOrder:       t,
					DurationMinutes: t * 60,
					FeeAmount:       pick([]int{1000, 2000, 3000, 5000}) * t,
					IsLastTier:      t == tierCount,
				})
			}
		}
	}

	if len(randConfigs) > 0 {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&randConfigs).Error; err != nil {
			return err
		}
	}
	if len(randTiers) > 0 {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&randTiers).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedHolidayRates(db *gorm.DB) error {
	var existing int64
	db.Model(&feeDomain.HolidayRate{}).Count(&existing)
	if existing > 0 {
		return nil
	}

	adminID := deterministicUUID("seed:admin")
	rateTypes := []types.HolidayRateType{types.HolidayRateMultiplier, types.HolidayRateOverride}
	names := []string{"Hari Raya Idul Fitri", "Natal & Tahun Baru", "Lebaran", "Hari Kemerdekaan", "Hari Libur Nasional"}

	n := randBetween(5, 15)
	rates := make([]feeDomain.HolidayRate, 0, n)
	now := time.Now()

	for i := 0; i < n; i++ {
		start := gofakeit.DateRange(now, now.AddDate(1, 0, 0))
		end := start.AddDate(0, 0, gofakeit.IntRange(1, 7))
		rt := rateTypes[rand.Intn(len(rateTypes))]

		hr := feeDomain.HolidayRate{
			ID:        uuid.New(),
			Name:      names[rand.Intn(len(names))],
			DateStart: start,
			DateEnd:   end,
			RateType:  rt,
			CreatedBy: &adminID,
		}
		if rt == types.HolidayRateMultiplier {
			m := decimal.NewFromFloat(gofakeit.Float64Range(1.2, 2.0)).Round(2)
			hr.Multiplier = &m
		} else {
			f := pick([]int{5000, 10000, 15000, 20000})
			hr.OverrideFee = &f
		}
		rates = append(rates, hr)
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rates).Error
}

func seedOCRConfig(db *gorm.DB) error {
	var existing int64
	db.Model(&feeDomain.OCRConfig{}).Count(&existing)
	if existing > 0 {
		return nil
	}
	adminID := deterministicUUID("seed:admin")
	return db.Create(&feeDomain.OCRConfig{
		ID:                  deterministicUUID("ocr:config:default"),
		AutoAcceptThreshold: decimal.NewFromFloat(0.85),
		IsActive:            true,
		EffectiveFrom:       time.Now().AddDate(-1, 0, 0),
		CreatedBy:           &adminID,
	}).Error
}

func seedOverrideConfig(db *gorm.DB) error {
	var existing int64
	db.Model(&feeDomain.OverrideConfig{}).Count(&existing)
	if existing > 0 {
		return nil
	}
	adminID := deterministicUUID("seed:admin")
	notifyID := adminID
	return db.Create(&feeDomain.OverrideConfig{
		ID:                     deterministicUUID("override:config:default"),
		MaxOverridesPerDay:     10,
		MaxOverridesPerWeek:    30,
		EscalationNotifyUserID: &notifyID,
		IsActive:               true,
		CreatedBy:              &adminID,
	}).Error
}

// ── transaction ───────────────────────────────────────────────────────────────

func seedTransactions(db *gorm.DB) error {
	var existing int64
	db.Model(&txDomain.Transaction{}).Count(&existing)
	if existing > 0 {
		return nil
	}

	var entryGates []zoneDomain.Gate
	db.Where("gate_type = ?", types.GateTypeEntry).Select("id, zone_id").Find(&entryGates)

	var exitGates []zoneDomain.Gate
	db.Where("gate_type = ?", types.GateTypeExit).Select("id, zone_id").Find(&exitGates)

	var vehicles []vehicleDomain.Vehicle
	db.Select("id").Find(&vehicles)

	var rfidCards []rfidDomain.RFIDCard
	db.Where("is_active = ?", true).Select("id").Find(&rfidCards)

	if len(entryGates) == 0 {
		return nil
	}

	// Build exit gate lookup by zone_id for realistic pairing
	exitByZone := map[uuid.UUID][]uuid.UUID{}
	for _, g := range exitGates {
		exitByZone[g.ZoneID] = append(exitByZone[g.ZoneID], g.ID)
	}

	statusPool := []types.TransactionStatus{
		types.TransactionStatusExited, types.TransactionStatusExited,
		types.TransactionStatusExited, types.TransactionStatusPaid,
		types.TransactionStatusAwaitingPayment, types.TransactionStatusOpen,
		types.TransactionStatusOverridden, types.TransactionStatusCancelled,
	}
	entryMethods := []types.EntryMethod{types.EntryMethodRFID, types.EntryMethodQR}
	exitMethods := []types.ExitMethod{types.ExitMethodRFID, types.ExitMethodQR, types.ExitMethodOverride}

	n := randBetween(25, 80)
	transactions := make([]txDomain.Transaction, 0, n)
	logs := make([]txDomain.TransactionLog, 0, n)

	for i := 0; i < n; i++ {
		eg := entryGates[rand.Intn(len(entryGates))]
		entryAt := gofakeit.DateRange(time.Now().AddDate(0, -3, 0), time.Now())
		status := statusPool[rand.Intn(len(statusPool))]
		method := entryMethods[rand.Intn(len(entryMethods))]

		tx := txDomain.Transaction{
			ID:              uuid.New(),
			TransactionCode: fmt.Sprintf("PKR-%s-%05d", entryAt.Format("20060102"), i+1),
			EntryGateID:     eg.ID,
			ZoneID:          eg.ZoneID,
			EntryMethod:     method,
			EntryAt:         entryAt,
			Status:          status,
		}

		if method == types.EntryMethodRFID && len(rfidCards) > 0 {
			cid := rfidCards[rand.Intn(len(rfidCards))].ID
			tx.RFIDCardID = &cid
		} else {
			qr := gofakeit.UUID()
			tx.EntryQRCode = &qr
		}

		if len(vehicles) > 0 && gofakeit.Bool() {
			vid := vehicles[rand.Intn(len(vehicles))].ID
			tx.VehicleID = &vid
		}

		// Closed statuses get exit data
		if status == types.TransactionStatusExited ||
			status == types.TransactionStatusPaid ||
			status == types.TransactionStatusOverridden {
			exitAt := entryAt.Add(time.Duration(gofakeit.IntRange(10, 480)) * time.Minute)
			tx.ExitAt = &exitAt
			em := exitMethods[rand.Intn(len(exitMethods))]
			tx.ExitMethod = &em
			fee := pick([]int{2000, 3000, 5000, 8000, 10000, 15000, 20000})
			tx.CalculatedFee = &fee

			if zoneExits, ok := exitByZone[eg.ZoneID]; ok {
				xgID := zoneExits[rand.Intn(len(zoneExits))]
				tx.ExitGateID = &xgID
			}
		}

		transactions = append(transactions, tx)

		logs = append(logs, txDomain.TransactionLog{
			ID:            uuid.New(),
			TransactionID: tx.ID,
			ToStatus:      string(types.TransactionStatusOpen),
			Event:         types.EventEntryCreated,
			TriggeredBy:   types.TriggeredBySystem,
			CreatedAt:     entryAt,
		})
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&transactions).Error; err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&logs).Error
}

// ── payment ───────────────────────────────────────────────────────────────────

func seedPayments(db *gorm.DB) error {
	var existing int64
	db.Model(&paymentDomain.Payment{}).Count(&existing)
	if existing > 0 {
		return nil
	}

	var txs []txDomain.Transaction
	db.Where("status IN ?", []types.TransactionStatus{
		types.TransactionStatusPaid,
		types.TransactionStatusExited,
	}).Select("id, calculated_fee, entry_at").Find(&txs)

	if len(txs) == 0 {
		return nil
	}

	methods := []types.PaymentMethod{types.PaymentMethodCash, types.PaymentMethodQRIS}
	payments := make([]paymentDomain.Payment, 0, len(txs))

	for _, tx := range txs {
		fee := 5000
		if tx.CalculatedFee != nil {
			fee = *tx.CalculatedFee
		}
		method := methods[rand.Intn(len(methods))]
		paidAt := tx.EntryAt.Add(time.Duration(gofakeit.IntRange(1, 30)) * time.Minute)

		p := paymentDomain.Payment{
			ID:            uuid.New(),
			TransactionID: tx.ID,
			Method:        method,
			Amount:        fee,
			Status:        types.PaymentStatusCompleted,
			PaidAt:        &paidAt,
		}
		if method == types.PaymentMethodCash {
			tendered := roundUpToNearest(fee, 5000)
			change := tendered - fee
			p.CashTendered = &tendered
			p.CashChange = &change
		} else {
			orderID := "MID-" + gofakeit.UUID()
			p.MidtransOrderID = &orderID
		}
		payments = append(payments, p)
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&payments).Error
}

// ── override ──────────────────────────────────────────────────────────────────

func seedOperatorOverrides(db *gorm.DB) error {
	var existing int64
	db.Model(&overrideDomain.OperatorOverride{}).Count(&existing)
	if existing > 0 {
		return nil
	}

	var txs []txDomain.Transaction
	db.Where("status = ?", types.TransactionStatusOverridden).Select("id").Find(&txs)

	var operatorUsers []authDomain.User
	db.Joins("JOIN roles ON roles.id = users.role_id").
		Where("roles.name = ?", types.RoleOperator).
		Select("users.id").Find(&operatorUsers)

	adminID := deterministicUUID("seed:admin")
	if len(txs) == 0 || len(operatorUsers) == 0 {
		return nil
	}

	overrideTypes := []types.OverrideType{
		types.OverrideLostCardExit,
		types.OverrideNoQRExit,
		types.OverrideFeeWaive,
		types.OverrideFeeAdjust,
		types.OverrideForceOpenGate,
		types.OverrideManualEntry,
	}

	overrides := make([]overrideDomain.OperatorOverride, 0, len(txs))
	for _, tx := range txs {
		op := operatorUsers[rand.Intn(len(operatorUsers))]
		origFee := pick([]int{5000, 10000, 15000})
		adjFee := pick([]int{0, 2000, 5000})
		overrides = append(overrides, overrideDomain.OperatorOverride{
			ID:            uuid.New(),
			TransactionID: tx.ID,
			OperatorID:    op.ID,
			OverrideType:  overrideTypes[rand.Intn(len(overrideTypes))],
			Reason:        gofakeit.Sentence(8),
			OriginalFee:   &origFee,
			AdjustedFee:   &adjFee,
			ApprovedBy:    adminID,
		})
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&overrides).Error
}

// ── ocr ───────────────────────────────────────────────────────────────────────

func seedOCRJobsAndResults(db *gorm.DB) error {
	var existing int64
	db.Model(&ocrDomain.OCRJob{}).Count(&existing)
	if existing > 0 {
		return nil
	}

	var txs []txDomain.Transaction
	db.Select("id", "entry_at").Find(&txs)
	if len(txs) == 0 {
		return nil
	}

	statusPool := []types.OCRJobStatus{
		types.OCRJobCompleted, types.OCRJobCompleted, types.OCRJobCompleted,
		types.OCRJobFailed, types.OCRJobSkipped,
	}
	seenPlates := map[string]bool{}

	jobs := make([]ocrDomain.OCRJob, 0, len(txs))
	results := make([]ocrDomain.OCRResult, 0, len(txs))

	for _, tx := range txs {
		jobStatus := statusPool[rand.Intn(len(statusPool))]
		queuedAt := tx.EntryAt
		startedAt := queuedAt.Add(2 * time.Second)
		completedAt := startedAt.Add(time.Duration(gofakeit.IntRange(1, 5)) * time.Second)

		job := ocrDomain.OCRJob{
			ID:            uuid.New(),
			TransactionID: tx.ID,
			ImageURL:      fmt.Sprintf("https://storage.parkieee.local/ocr/%s.jpg", tx.ID),
			Status:        jobStatus,
			RetryCount:    gofakeit.IntRange(0, 2),
			QueuedAt:      queuedAt,
			StartedAt:     &startedAt,
		}
		if jobStatus == types.OCRJobCompleted || jobStatus == types.OCRJobFailed {
			job.CompletedAt = &completedAt
		}
		jobs = append(jobs, job)

		if jobStatus == types.OCRJobCompleted {
			confidence := decimal.NewFromFloat(gofakeit.Float64Range(0.70, 0.99)).Round(4)
			plate := uniquePlate(seenPlates)
			seenPlates[plate] = true
			verifiedAt := completedAt.Add(1 * time.Second)
			isVerified := confidence.GreaterThanOrEqual(decimal.NewFromFloat(0.85))

			results = append(results, ocrDomain.OCRResult{
				ID:            uuid.New(),
				OCRJobID:      job.ID,
				PlateDetected: plate,
				Confidence:    confidence,
				RawOutput:     datatypes.JSON(fmt.Sprintf(`{"plate":"%s","confidence":%s}`, plate, confidence.String())),
				IsVerified:    isVerified,
				VerifiedAt:    &verifiedAt,
			})
		}
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&jobs).Error; err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&results).Error
}

// ── audit ─────────────────────────────────────────────────────────────────────

func seedAuditLogs(db *gorm.DB) error {
	var existing int64
	db.Model(&auditDomain.AuditLog{}).Count(&existing)
	if existing > 0 {
		return nil
	}

	var users []authDomain.User
	db.Select("id, role_id").Find(&users)
	if len(users) == 0 {
		return nil
	}

	// role name lookup
	var roles []authDomain.Role
	db.Select("id, name").Find(&roles)
	roleNameByID := map[uuid.UUID]string{}
	for _, r := range roles {
		roleNameByID[r.ID] = r.Name
	}

	eventTypes := []types.AuditEventType{
		types.AuditFeeConfigChanged, types.AuditRoleAssigned, types.AuditPermissionChanged,
		types.AuditOverridePerformed, types.AuditCardDeactivated, types.AuditUserCreated,
		types.AuditUserDeactivated, types.AuditHolidayRateChanged, types.AuditOCRConfigChanged,
	}
	targetTypes := []types.AuditTargetType{
		types.AuditTargetFeeConfig, types.AuditTargetUser, types.AuditTargetRole,
		types.AuditTargetRFIDCard, types.AuditTargetHolidayRate, types.AuditTargetOCRConfig,
	}

	n := randBetween(25, 80)
	logs := make([]auditDomain.AuditLog, 0, n)

	for i := 0; i < n; i++ {
		actor := users[rand.Intn(len(users))]
		roleName := roleNameByID[actor.RoleID]
		if roleName == "" {
			roleName = "operator"
		}
		targetID := uuid.New()

		logs = append(logs, auditDomain.AuditLog{
			ID:          uuid.New(),
			EventType:   eventTypes[rand.Intn(len(eventTypes))],
			ActorID:     actor.ID,
			ActorRole:   roleName,
			TargetType:  targetTypes[rand.Intn(len(targetTypes))],
			TargetID:    &targetID,
			BeforeState: datatypes.JSON(`{"state":"before"}`),
			AfterState:  datatypes.JSON(`{"state":"after"}`),
			IPAddress:   gofakeit.IPv4Address(),
			UserAgent:   gofakeit.UserAgent(),
			CreatedAt:   gofakeit.DateRange(time.Now().AddDate(0, -1, 0), time.Now()),
		})
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&logs).Error
}

// ── helpers ───────────────────────────────────────────────────────────────────

var seedNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

func deterministicUUID(name string) uuid.UUID { return uuid.NewSHA1(seedNamespace, []byte(name)) }
func roleID(name string) uuid.UUID            { return deterministicUUID("role:" + name) }
func permID(node string) uuid.UUID            { return deterministicUUID("perm:" + node) }

func randBetween(min, max int) int { return min + rand.Intn(max-min+1) }

func pick[T any](slice []T) T { return slice[rand.Intn(len(slice))] }

func uniqueEmail(seen map[string]bool) string {
	for {
		e := strings.ToLower(fmt.Sprintf("%s.%s%d@%s",
			gofakeit.FirstName(), gofakeit.LastName(),
			gofakeit.IntRange(1, 99), gofakeit.DomainName(),
		))
		if !seen[e] {
			return e
		}
	}
}

func uniquePlate(seen map[string]bool) string {
	prefixes := []string{"B", "D", "F", "H", "L", "N", "AB", "AD", "AE", "AG", "BK", "BM", "BG"}
	for {
		p := fmt.Sprintf("%s %d %s",
			prefixes[rand.Intn(len(prefixes))],
			gofakeit.IntRange(1000, 9999),
			strings.ToUpper(gofakeit.Lexify("???")),
		)
		if !seen[p] {
			return p
		}
	}
}

func uniqueCardUID(seen map[string]bool) string {
	for {
		uid := strings.ToUpper(fmt.Sprintf("%s:%s:%s:%s",
			gofakeit.Lexify("??"), gofakeit.Lexify("??"),
			gofakeit.Lexify("??"), gofakeit.Lexify("??"),
		))
		if !seen[uid] {
			return uid
		}
	}
}

func uniqueZoneName(seen map[string]bool, words, labels []string) string {
	for {
		name := fmt.Sprintf("%s %s", words[rand.Intn(len(words))], labels[rand.Intn(len(labels))])
		if !seen[name] {
			return name
		}
		name = fmt.Sprintf("%s %s-%d", words[rand.Intn(len(words))], labels[rand.Intn(len(labels))], gofakeit.IntRange(2, 9))
		if !seen[name] {
			return name
		}
	}
}

// roundUpToNearest rounds n up to the nearest multiple of step (for cash change calc).
func roundUpToNearest(n, step int) int {
	if n%step == 0 {
		return n
	}
	return n + step - n%step
}
