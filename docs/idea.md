{
  "WTGLogEntry": {
    "logId": "string",
    "wtgId": "string",
    "dateTimeUTC": "ISO8601",
    "activityType": "Inspection | Preventive | Corrective | RemoteIntervention | SoftwareUpdate | ParameterChange",
    "workOrderId": "string",
    "trigger": "AlarmId | ScheduleId | RCARecommendation | InspectionFinding",
    "personnel": ["string"],
    "proceduresUsed": ["DocRef"],
    "findings": "string",
    "partsUsed": [
      {
        "partId": "string",
        "serialOrBatch": "string",
        "quantity": 1
      }
    ],
    "testResults": ["DocRef"],
    "photos": [
      {
        "uri": "string",
        "geoTag": "lat,lon",
        "resolution": "pixels"
      }
    ],
    "signOff": {
      "by": "string",
      "dateTimeUTC": "ISO8601",
      "version": "string"
    }
  },
  "MonthlyReport": {
    "siteId": "string",
    "period": "YYYY-MM",
    "availability": {
      "technical": "percent",
      "commercial": "percent"
    },
    "kpis": {
      "alarmsRaised": "number",
      "alarmsClosed": "number",
      "meanTimeToRepairHours": "number",
      "meanTimeBetweenFailuresHours": "number"
    },
    "events": [
      {
        "eventId": "string",
        "wtgId": "string",
        "startUTC": "ISO8601",
        "endUTC": "ISO8601",
        "category": "Planned | Unplanned",
        "rootCause": "string",
        "downtimeHours": "number",
        "vesselTimeHours": "number"
      }
    ],
    "spares": [
      {
        "partId": "string",
        "category": "Major (Y-A.1) | Minor (Y-A.2)",
        "quantityConsumed": "number",
        "remainingStock": "number",
        "expiryDate": "YYYY-MM-DD"
      }
    ],
    "bim": {
      "inspections": "number",
      "findingsByClass": {
        "M": "number",
        "M-Nx": "number",
        "RM-1": "number",
        "RM-2": "number"
      }
    }
  },
  "GoldenParameters": {
    "wtgModel": "string",
    "swBaseline": "string",
    "parameters": [
      {
        "name": "string",
        "value": "number|string|boolean",
        "units": "string",
        "tolerance": "±value or range",
        "safetyCritical": true
      }
    ],
    "version": "string",
    "effectiveDate": date
    }
  }
}
    "effectiveDateUTC": "ISO8601"
  }
}

{
  "SparePartMaster": {
    "partNumber": "string",
    "revision": "string",
    "description": "string",
    "category": "Major | Minor | Consumable | Tooling | Safety",
    "criticality": "Critical | High | Medium | Low",
    "safetyCritical": true,
    "compatibleWTGModels": ["string"],
    "alternates": ["partNumberRev"],
    "supersedes": "partNumberRev",
    "supersededBy": "partNumberRev",
    "uom": "EA",
    "hsCode": "string",
    "countryOfOrigin": "string",
    "rohsReach": "compliant | exempt",
    "hazardClass": "ADR/IMDG/IATA code",
    "storageConditions": {
      "temperatureC": "min..max",
      "humidityRH": "max%",
      "ESD": "ESD-safe required",
      "other": "string"
    },
    "shelfLifeDays": 365,
    "leadTimeDays": 60,
    "moq": 1,
    "stdPack": 1,
    "netWeightKg": 0.0,
    "dimsMm": { "l": 0, "w": 0, "h": 0 },
    "warranty": { "termMonths": 24, "startPoint": "Delivery | Installation" },
    "docsRequired": ["CoC", "SDS", "TestReport", "CalibrationCert"]
  },
  "InventoryPolicy": {
    "partNumber": "string",
    "targetServiceLevelPct": 97,
    "avgDemandPerMonth": 2.5,
    "demandStdDev": 1.1,
    "leadTimeDays": 60,
    "safetyStock": 5,
    "reorderPoint": 10,
    "minLevel": 8,
    "maxLevel": 20,
    "reviewCycleDays": 30,
    "fefo": true
  },
  "LeadTimeMonitor": {
    "partNumber": "string",
    "stdLeadTimeDays": 60,
    "currentLeadTimeDays": 75,
    "thresholdBreach": true,
    "expediteOptions": ["PremiumAir", "AltSupplier"],
    "nextReviewUTC": "2025-10-01T00:00:00Z"
  }
}
