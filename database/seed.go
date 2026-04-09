package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
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

func Truncate(db *gorm.DB) error {
	tables := []string{
		"audit_log_exports",
		"audit_logs",
		"ocr_review_logs",
		"ocr_results",
		"ocr_jobs",
		"operator_overrides",
		"refunds",
		"midtrans_callbacks",
		"payments",
		"unclosed_transaction_flags",
		"transaction_logs",
		"transactions",
		"gate_pairing_codes",
		"zone_capacity_logs",
		"gate_devices",
		"gates",
		"zones",
		"override_configs",
		"ocr_configs",
		"holiday_rates",
		"fee_tiers",
		"fee_configs",
		"rfid_cards",
		"vehicles",
		"vehicle_types",
		"role_permissions",
		"user_login_stats",
		"user_login_logs",
		"user_sessions",
		"users",
		"permissions",
		"roles",
	}

	for _, t := range tables {
		if err := db.Exec("TRUNCATE TABLE \"" + t + "\" RESTART IDENTITY CASCADE").Error; err != nil {
			return fmt.Errorf("truncate %s: %w", t, err)
		}
	}
	return nil
}

func Seed(db *gorm.DB) error {
	gofakeit.Seed(time.Now().UnixNano())

	steps := []struct {
		name string
		fn   func(*gorm.DB) error
	}{
		{"roles", seedRoles},
		{"permissions", seedPermissions},
		{"role_permissions", seedRolePermissions},
		{"hardcoded users", seedHardcodedUsers},
		{"random users", seedUsers},
		{"vehicle_types", seedVehicleTypes},
		{"vehicles", seedVehicles},
		{"zones + gates", seedZonesAndGates},
		{"rfid_cards", seedRFIDCards},
		{"fee_configs + tiers", seedFeeConfigsAndTiers},
		{"holiday_rates", seedHolidayRates},
		{"ocr_config", seedOCRConfig},
		{"override_config", seedOverrideConfig},
		{"transactions", seedTransactions},
		{"zone_capacity_logs", seedZoneCapacityLogs},
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

func seedRoles(db *gorm.DB) error {
	roles := []authDomain.Role{
		{ID: roleID("operator"), Name: string(types.RoleOperator), Description: "Gate operator — handles entry/exit and overrides"},
		{ID: roleID("admin"), Name: string(types.RoleAdmin), Description: "Administrator — manages users, zones, and fee configs"},
		{ID: roleID("owner"), Name: string(types.RoleOwner), Description: "Business owner — full read access + holiday rate management"},
		{ID: roleID("engineer"), Name: string(types.RoleEngineer), Description: "Engineer — audit log access and system configuration"},
		{ID: roleID("cashier"), Name: string(types.RoleCashier), Description: "Cashier — handles payments at the exit gate"},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&roles).Error
}

func seedPermissions(db *gorm.DB) error {
	perms := []authDomain.Permission{
		// Gate
		{ID: permID("gate.override"), Node: string(types.PermGateOverride), Description: "Force open gate or override exit without ticket"},
		{ID: permID("gate.manage"), Node: string(types.PermGateManage), Description: "Create, edit, deactivate gate; regenerate gate token"},
		{ID: permID("gate.pair"), Node: string(types.PermGatePair), Description: "Confirm gate pairing and assign cashier to exit gate"},
		{ID: permID("gate.view"), Node: string(types.PermGateView), Description: "View gate list and active status"},
		// Zone
		{ID: permID("zone.manage"), Node: string(types.PermZoneManage), Description: "Create, edit, deactivate zone"},
		{ID: permID("zone.view"), Node: string(types.PermZoneView), Description: "View zone list and occupancy"},
		// Fee
		{ID: permID("fee.edit"), Node: string(types.PermFeeEdit), Description: "Create and modify fee configs, tiers, and holiday rates"},
		{ID: permID("fee.view"), Node: string(types.PermFeeView), Description: "View fee configs and tariff structure"},
		// User
		{ID: permID("user.manage"), Node: string(types.PermUserManage), Description: "Create, deactivate, and assign roles to users"},
		{ID: permID("user.view"), Node: string(types.PermUserView), Description: "View user list and profile"},
		{ID: permID("role.manage"), Node: string(types.PermRoleManage), Description: "Manage system permissions and role matrix"},
		{ID: permID("cashier.assign"), Node: string(types.PermCashierAssign), Description: "Assign or unassign cashier to an exit gate"},
		// Cashier
		{ID: permID("cashier.ability"), Node: string(types.PermCashierAbility), Description: "Access cashier station and receive payment requests from kiosk"},
		{ID: permID("payment.cash"), Node: string(types.PermPaymentCash), Description: "Process and confirm cash payments"},
		{ID: permID("payment.qris"), Node: string(types.PermPaymentQRIS), Description: "Initiate and handle QRIS payments"},
		{ID: permID("payment.refund"), Node: string(types.PermPaymentRefund), Description: "Request, approve, or reject payment refunds"},
		// Transaction
		{ID: permID("transaction.view"), Node: string(types.PermTransactionView), Description: "View transaction detail and history"},
		{ID: permID("transaction.cancel"), Node: string(types.PermTransactionCancel), Description: "Cancel an open transaction"},
		// Override
		{ID: permID("override.perform"), Node: string(types.PermOverridePerform), Description: "Perform overrides: lost card, no QR, fee waive, fee adjust, manual entry"},
		{ID: permID("override.config"), Node: string(types.PermOverrideConfig), Description: "Set override daily/weekly limit and escalation config"},
		// RFID
		{ID: permID("rfid.manage"), Node: string(types.PermRFIDManage), Description: "Deactivate RFID cards and link/unlink to vehicle"},
		{ID: permID("rfid.view"), Node: string(types.PermRFIDView), Description: "View RFID card registry"},
		// Report & system
		{ID: permID("report.view"), Node: string(types.PermReportView), Description: "View revenue reports and occupancy statistics"},
		{ID: permID("config.edit"), Node: string(types.PermConfigEdit), Description: "Edit system configs: OCR threshold, override limits"},
		{ID: permID("audit.read"), Node: string(types.PermAuditRead), Description: "Read and export audit logs"},
		// Internal service-to-service
		{ID: permID("internal.access"), Node: string(types.PermInternalAccess), Description: "Internal service token — used by Python object detection to call mark-exited"},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&perms).Error
}

func seedRolePermissions(db *gorm.DB) error {
	matrix := map[string][]string{
		"operator": {
			"gate.override",
			"gate.view",
			"gate.pair",
			"zone.view",
			"fee.view",
			"transaction.view",
			"transaction.cancel",
			"override.perform",
			"rfid.view",
			"audit.read",
			"report.view",
		},
		"admin": {
			"gate.override",
			"gate.manage",
			"gate.pair",
			"gate.view",
			"zone.manage",
			"zone.view",
			"fee.edit",
			"fee.view",
			"user.manage",
			"user.view",
			"role.manage",
			"cashier.assign",
			"cashier.ability",
			"payment.cash",
			"payment.qris",
			"payment.refund",
			"transaction.view",
			"transaction.cancel",
			"override.perform",
			"override.config",
			"rfid.manage",
			"rfid.view",
			"report.view",
			"config.edit",
			"audit.read",
		},
		"owner": {
			"gate.view",
			"zone.view",
			"zone.manage",
			"fee.edit",
			"fee.view",
			"user.view",
			"transaction.view",
			"report.view",
			"config.edit",
			"audit.read",
			"rfid.view",
			"payment.refund",
		},
		"engineer": {
			"gate.view",
			"zone.view",
			"gate.manage",
			"gate.pair",
			"zone.manage",
			"transaction.view",
			"role.manage",
			"audit.read",
			"config.edit",
		},
		"cashier": {
			"cashier.ability",
			"payment.cash",
			"payment.qris",
			"transaction.view",
			"fee.view",
		},
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

func seedHardcodedUsers(db *gorm.DB) error {
	type hardcodedUser struct {
		Name     string
		Username string
		Email    string
		Password string
		Role     string
	}

	accounts := []hardcodedUser{
		{Name: "System Administrator", Username: "admin", Email: "admin@parkieee.local", Password: "Admin@123!", Role: "admin"},
		{Name: "Bakayaro Operator", Username: "bakayaro", Email: "bakayaro@email.local", Password: "tripleT123", Role: "operator"},
		{Name: "Parkieee Owner", Username: "owner", Email: "owner@parkieee.local", Password: "Owner@123!", Role: "owner"},
		{Name: "System Engineer", Username: "engineer", Email: "engineer@parkieee.local", Password: "Engineer@123!", Role: "engineer"},
		{Name: "Main Cashier", Username: "cashier", Email: "cashier@parkieee.local", Password: "Cashier@123!", Role: "cashier"},
	}

	for _, acc := range accounts {
		var count int64
		db.Model(&authDomain.User{}).Where("email = ?", acc.Email).Count(&count)
		if count > 0 {
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(acc.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password for %s: %w", acc.Email, err)
		}

		if err := db.Create(&authDomain.User{
			ID:           uuid.New(),
			Name:         acc.Name,
			Username:     acc.Username,
			Email:        acc.Email,
			PasswordHash: string(hash),
			RoleID:       roleID(acc.Role),
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("create hardcoded user %s: %w", acc.Email, err)
		}
	}

	return nil
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

	roleNames := []string{"operator", "operator", "operator", "owner", "engineer", "cashier", "cashier"}
	n := randBetween(25, 80)
	users := make([]authDomain.User, 0, n)
	seenEmails := map[string]bool{"admin@parkieee.local": true}

	seenUsernames := map[string]bool{}
	for i := 0; i < n; i++ {
		email := uniqueEmail(seenEmails)
		seenEmails[email] = true
		username := uniqueUsername(seenUsernames)
		seenUsernames[username] = true
		users = append(users, authDomain.User{
			ID:           uuid.New(),
			Name:         gofakeit.Name(),
			Username:     username,
			Email:        email,
			PasswordHash: string(hash),
			RoleID:       roleID(roleNames[mathrand.Intn(len(roleNames))]),
			IsActive:     gofakeit.Bool(),
		})
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&users).Error
}

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
			VehicleTypeID: vtypePool[mathrand.Intn(len(vtypePool))],
			Source:        sources[mathrand.Intn(len(sources))],
			Notes:         gofakeit.RandomString([]string{"", "", gofakeit.Sentence(4)}),
		})
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&vehicles).Error
}

func seedZonesAndGates(db *gorm.DB) error {
	var existing int64
	db.Model(&zoneDomain.Zone{}).Count(&existing)
	if existing > 0 {
		vtMotorcycleFix := deterministicUUID("vtype:motorcycle")
		vtCarFix := deterministicUUID("vtype:car")
		db.Model(&zoneDomain.Zone{}).Where("id = ? AND for_vehicle_type_id IS NULL", deterministicUUID("zone:motor")).
			Update("for_vehicle_type_id", vtMotorcycleFix)
		db.Model(&zoneDomain.Zone{}).Where("id IN ? AND for_vehicle_type_id IS NULL",
			[]uuid.UUID{deterministicUUID("zone:mobil"), deterministicUUID("zone:vip")}).
			Update("for_vehicle_type_id", vtCarFix)
		db.Model(&zoneDomain.Zone{}).Where("for_vehicle_type_id IS NULL").
			Update("for_vehicle_type_id", vtMotorcycleFix)

		adminIDBackfill := deterministicUUID("seed:admin")
		var zonesWithoutGates []zoneDomain.Zone
		db.Raw(`SELECT z.* FROM zones z LEFT JOIN gates g ON g.zone_id = z.id WHERE g.id IS NULL`).Scan(&zonesWithoutGates)
		if len(zonesWithoutGates) > 0 {
			locationDescsBackfill := []string{
				"Pintu utara", "Pintu selatan", "Pintu timur", "Pintu barat",
				"Pintu basement", "Pintu utama", "Pintu samping", "Pintu darurat",
			}
			seenTokensBackfill := map[string]bool{}
			var backfillGates []zoneDomain.Gate
			for _, z := range zonesWithoutGates {
				backfillGates = append(backfillGates,
					zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Masuk", GateType: types.GateTypeEntry, LocationDesc: pick(locationDescsBackfill), GateToken: uniqueToken(seenTokensBackfill), IsActive: gofakeit.Bool(), CreatedBy: &adminIDBackfill},
					zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Keluar", GateType: types.GateTypeExit, LocationDesc: pick(locationDescsBackfill), GateToken: uniqueToken(seenTokensBackfill), IsActive: gofakeit.Bool(), CreatedBy: &adminIDBackfill},
				)
			}
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&backfillGates).Error; err != nil {
				return err
			}
		}
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
		{ID: deterministicUUID("zone:vip"), Name: "Parkir VIP", Description: "Area parkir VIP covered basement", Capacity: 20, AdditionalFee: 5000, ForVehicleTypeID: &vtCar, IsActive: false, CreatedBy: &adminID},
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
		vtID := vtPool[mathrand.Intn(len(vtPool))]
		extra = append(extra, zoneDomain.Zone{
			ID:               uuid.New(),
			Name:             name,
			Description:      gofakeit.Sentence(6),
			Capacity:         gofakeit.IntRange(10, 300),
			AdditionalFee:    feeOptions[mathrand.Intn(len(feeOptions))],
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

	seenTokens := map[string]bool{}
	var gates []zoneDomain.Gate
	for i, z := range allZones {
		// Skip the last zone to leave it without gates (for testing zone-gate pairing UI)
		if i == len(allZones)-1 {
			continue
		}
		gates = append(gates,
			zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Masuk", GateType: types.GateTypeEntry, LocationDesc: pick(locationDescs), GateToken: uniqueToken(seenTokens), IsActive: gofakeit.Bool(), CreatedBy: &adminID},
			zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Keluar", GateType: types.GateTypeExit, LocationDesc: pick(locationDescs), GateToken: uniqueToken(seenTokens), IsActive: gofakeit.Bool(), CreatedBy: &adminID},
		)
		if gofakeit.Bool() {
			gates = append(gates,
				zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Masuk 2", GateType: types.GateTypeEntry, LocationDesc: pick(locationDescs), GateToken: uniqueToken(seenTokens), IsActive: gofakeit.Bool(), CreatedBy: &adminID},
				zoneDomain.Gate{ID: uuid.New(), ZoneID: z.ID, Name: z.Name + " - Keluar 2", GateType: types.GateTypeExit, LocationDesc: pick(locationDescs), GateToken: uniqueToken(seenTokens), IsActive: gofakeit.Bool(), CreatedBy: &adminID},
			)
		}
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&gates).Error
}

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
			vid := vehicleIDs[mathrand.Intn(len(vehicleIDs))]
			card.VehicleID = &vid
		}
		cards = append(cards, card)
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&cards).Error
}

func seedFeeConfigsAndTiers(db *gorm.DB) error {
	adminID := deterministicUUID("seed:admin")
	now := time.Now()

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
		rt := rateTypes[mathrand.Intn(len(rateTypes))]

		hr := feeDomain.HolidayRate{
			ID:        uuid.New(),
			Name:      names[mathrand.Intn(len(names))],
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
		eg := entryGates[mathrand.Intn(len(entryGates))]
		
		// 30% chance for today, 70% for past 3 months
		var entryAt time.Time
		if mathrand.Float32() < 0.3 {
			entryAt = gofakeit.DateRange(time.Now().Truncate(24*time.Hour), time.Now())
		} else {
			entryAt = gofakeit.DateRange(time.Now().AddDate(0, -3, 0), time.Now().Add(-24*time.Hour))
		}

		status := statusPool[mathrand.Intn(len(statusPool))]
		method := entryMethods[mathrand.Intn(len(entryMethods))]

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
			cid := rfidCards[mathrand.Intn(len(rfidCards))].ID
			tx.RFIDCardID = &cid
		} else {
			qr := gofakeit.UUID()
			tx.EntryQRCode = &qr
		}

		if len(vehicles) > 0 && gofakeit.Bool() {
			vid := vehicles[mathrand.Intn(len(vehicles))].ID
			tx.VehicleID = &vid
		}

		if status == types.TransactionStatusExited ||
			status == types.TransactionStatusPaid ||
			status == types.TransactionStatusOverridden {
			exitAt := entryAt.Add(time.Duration(gofakeit.IntRange(10, 480)) * time.Minute)
			tx.ExitAt = &exitAt
			em := exitMethods[mathrand.Intn(len(exitMethods))]
			tx.ExitMethod = &em
			fee := pick([]int{2000, 3000, 5000, 8000, 10000, 15000, 20000})
			tx.CalculatedFee = &fee

			if zoneExits, ok := exitByZone[eg.ZoneID]; ok {
				xgID := zoneExits[mathrand.Intn(len(zoneExits))]
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

func seedZoneCapacityLogs(db *gorm.DB) error {
	var zones []zoneDomain.Zone
	db.Find(&zones)

	for _, z := range zones {
		var txs []txDomain.Transaction
		db.Where("zone_id = ? AND status = ?", z.ID, types.TransactionStatusOpen).
			Order("entry_at ASC").
			Find(&txs)

		occupied := 0
		for _, tx := range txs {
			occupied++
			db.Create(&zoneDomain.ZoneCapacityLog{
				ID:             uuid.New(),
				ZoneID:         z.ID,
				TransactionID:  tx.ID,
				EventType:      types.ZoneEventEntry,
				OccupiedCount:  occupied,
				AvailableCount: z.Capacity - occupied,
				RecordedAt:     tx.EntryAt,
			})
		}
	}
	return nil
}

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
		method := methods[mathrand.Intn(len(methods))]
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
		op := operatorUsers[mathrand.Intn(len(operatorUsers))]
		origFee := pick([]int{5000, 10000, 15000})
		adjFee := pick([]int{0, 2000, 5000})
		overrides = append(overrides, overrideDomain.OperatorOverride{
			ID:            uuid.New(),
			TransactionID: tx.ID,
			OperatorID:    op.ID,
			OverrideType:  overrideTypes[mathrand.Intn(len(overrideTypes))],
			Reason:        gofakeit.Sentence(8),
			OriginalFee:   &origFee,
			AdjustedFee:   &adjFee,
			ApprovedBy:    adminID,
		})
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&overrides).Error
}

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
		jobStatus := statusPool[mathrand.Intn(len(statusPool))]
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
		actor := users[mathrand.Intn(len(users))]
		roleName := roleNameByID[actor.RoleID]
		if roleName == "" {
			roleName = "operator"
		}
		targetID := uuid.New()

		logs = append(logs, auditDomain.AuditLog{
			ID:          uuid.New(),
			EventType:   eventTypes[mathrand.Intn(len(eventTypes))],
			ActorID:     actor.ID,
			ActorRole:   roleName,
			TargetType:  targetTypes[mathrand.Intn(len(targetTypes))],
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

var seedNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

func deterministicUUID(name string) uuid.UUID { return uuid.NewSHA1(seedNamespace, []byte(name)) }
func roleID(name string) uuid.UUID            { return deterministicUUID("role:" + name) }
func permID(node string) uuid.UUID            { return deterministicUUID("perm:" + node) }

func randBetween(min, max int) int { return min + mathrand.Intn(max-min+1) }

func pick[T any](slice []T) T { return slice[mathrand.Intn(len(slice))] }

func randomToken() string {
	b := make([]byte, 30)
	rand.Read(b)
	return hex.EncodeToString(b)
}

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

func uniqueToken(seen map[string]bool) string {
	for {
		t := randomToken()
		if !seen[t] {
			return t
		}
	}
}

func uniquePlate(seen map[string]bool) string {
	prefixes := []string{"B", "D", "F", "H", "L", "N", "AB", "AD", "AE", "AG", "BK", "BM", "BG"}
	for {
		p := fmt.Sprintf("%s %d %s",
			prefixes[mathrand.Intn(len(prefixes))],
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

func uniqueUsername(seen map[string]bool) string {
	for {
		u := strings.ToLower(fmt.Sprintf("%s%d", gofakeit.Username(), gofakeit.IntRange(1, 99)))
		if len(u) >= 3 && len(u) <= 50 && !seen[u] {
			return u
		}
	}
}

func uniqueZoneName(seen map[string]bool, words, labels []string) string {
	for {
		name := fmt.Sprintf("%s %s", words[mathrand.Intn(len(words))], labels[mathrand.Intn(len(labels))])
		if !seen[name] {
			return name
		}
		name = fmt.Sprintf("%s %s-%d", words[mathrand.Intn(len(words))], labels[mathrand.Intn(len(labels))], gofakeit.IntRange(2, 9))
		if !seen[name] {
			return name
		}
	}
}

func roundUpToNearest(n, step int) int {
	if n%step == 0 {
		return n
	}
	return n + step - n%step
}
