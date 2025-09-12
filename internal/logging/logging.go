package logging

import (
    "context"
    "fmt"
    "io"
    "log/slog"
    "os"
    "sort"
    "strings"
    "sync"
    "time"
    "unicode/utf8"
    
    "yourapp/internal/auth"
    "yourapp/internal/middleware"
)

const timeLayout = "2006/01/02 15:04:05"

// customTextHandler writes lines like:
// 2025/09/06 21:11:44 level=INFO msg="starting" key=value ...
type customTextHandler struct {
    out        io.Writer
    mu         sync.Mutex
    minLevel   slog.Leveler
    attrs      []slog.Attr
    groups     []string
    timeLayout string
}

func (h *customTextHandler) Enabled(_ context.Context, l slog.Level) bool {
    min := slog.LevelInfo
    if h.minLevel != nil {
        min = h.minLevel.Level()
    }
    return l >= min
}

func upperLevel(l slog.Level) string {
    switch {
    case l <= slog.LevelDebug:
        return "DEBUG"
    case l <= slog.LevelInfo:
        return "INFO"
    case l <= slog.LevelWarn:
        return "WARN"
    default:
        return "ERROR"
    }
}

func needsQuoting(s string) bool {
    if s == "" {
        return true
    }
    for _, r := range s {
        if r <= ' ' || r == '"' || r == '=' || r == '\\' {
            return true
        }
        if !utf8.ValidRune(r) {
            return true
        }
    }
    return false
}

func quote(s string) string {
    b := &strings.Builder{}
    b.WriteByte('"')
    for i := 0; i < len(s); i++ {
        c := s[i]
        if c == '"' || c == '\\' {
            b.WriteByte('\\')
        }
        b.WriteByte(c)
    }
    b.WriteByte('"')
    return b.String()
}

func appendKeyVal(sb *strings.Builder, key string, val any) {
    sb.WriteByte(' ')
    sb.WriteString(key)
    sb.WriteByte('=')
    switch v := val.(type) {
    case string:
        if needsQuoting(v) {
            sb.WriteString(quote(v))
        } else {
            sb.WriteString(v)
        }
    case time.Duration:
        sb.WriteString(v.String())
    case fmt.Stringer:
        s := v.String()
        if needsQuoting(s) {
            sb.WriteString(quote(s))
        } else {
            sb.WriteString(s)
        }
    default:
        // Let fmt handle numbers, bools, etc.
        s := fmt.Sprint(v)
        if needsQuoting(s) {
            sb.WriteString(quote(s))
        } else {
            sb.WriteString(s)
        }
    }
}

func (h *customTextHandler) Handle(ctx context.Context, r slog.Record) error {
    ts := r.Time
    if ts.IsZero() {
        ts = time.Now()
    }
    var sb strings.Builder
    sb.Grow(256)
    // Timestamp prefix
    sb.WriteString(ts.Format(h.timeLayout))
    // Level
    sb.WriteString(" level=")
    sb.WriteString(upperLevel(r.Level))
    // Message
    if r.Message != "" {
        sb.WriteString(" msg=")
        sb.WriteString(quote(r.Message))
    }

    // Collect attrs (base + record) for ordered rendering.
    attrs := make([]slog.Attr, 0, len(h.attrs)+8)
    attrs = append(attrs, h.attrs...)
    r.Attrs(func(a slog.Attr) bool {
        attrs = append(attrs, a)
        return true
    })

    // Convert to simple forms; flatten groups into separate pairs.
    type pair struct{ k string; v any }
    normal := map[string]any{}
    groupsFlat := make([]pair, 0)

    // Enrich from context: request_id, user_id, org_id, provider
    if rid, ok := middleware.GetRequestID(ctx); ok {
        normal["request_id"] = rid
    }
    if uid, ok := middleware.GetLogUserID(ctx); ok {
        normal["user_id"] = uid
    }
    if oid, ok := middleware.GetLogOrgID(ctx); ok {
        normal["org_id"] = oid
    }
    if prov, ok := middleware.GetLogProvider(ctx); ok {
        normal["provider"] = prov
    }
    if sess, ok := auth.SessionFromContext(ctx); ok && sess != nil {
        normal["user_id"] = sess.UserID.String()
        normal["org_id"] = sess.ActiveOrg.String()
        if sess.Provider != "" { normal["provider"] = sess.Provider }
    } else if u, ok := auth.GetUserFromContext(ctx); ok && u != nil {
        normal["user_id"] = u.ID.String()
    }
    if org, ok := auth.OrgFromContext(ctx); ok {
        normal["org_id"] = org.String()
    }
    for _, a := range attrs {
        if a.Key == "" {
            continue
        }
        v := a.Value
        switch v.Kind() {
        case slog.KindString:
            normal[a.Key] = v.String()
        case slog.KindBool:
            normal[a.Key] = v.Bool()
        case slog.KindInt64:
            normal[a.Key] = v.Int64()
        case slog.KindUint64:
            normal[a.Key] = v.Uint64()
        case slog.KindFloat64:
            normal[a.Key] = v.Float64()
        case slog.KindDuration:
            normal[a.Key] = v.Duration()
        case slog.KindTime:
            normal[a.Key] = v.Time().Format(time.RFC3339)
        case slog.KindGroup:
            if g := v.Group(); len(g) > 0 {
                for _, ga := range g {
                    if ga.Key == "" {
                        continue
                    }
                    groupsFlat = append(groupsFlat, pair{k: a.Key + "." + ga.Key, v: ga.Value.Any()})
                }
            }
        default:
            normal[a.Key] = v.Any()
        }
    }

    // Priority keys printed first in this exact order if present.
    prio := []string{"method", "url", "status", "duration"}
    for _, k := range prio {
        if v, ok := normal[k]; ok {
            appendKeyVal(&sb, k, v)
            delete(normal, k)
        }
    }

    // Remaining normal keys sorted.
    keys := make([]string, 0, len(normal))
    for k := range normal {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    for _, k := range keys {
        appendKeyVal(&sb, k, normal[k])
    }

    // Finally, flattened group pairs sorted by key.
    sort.SliceStable(groupsFlat, func(i, j int) bool { return groupsFlat[i].k < groupsFlat[j].k })
    for _, p := range groupsFlat {
        appendKeyVal(&sb, p.k, p.v)
    }

    sb.WriteByte('\n')

    h.mu.Lock()
    defer h.mu.Unlock()
    _, err := h.out.Write([]byte(sb.String()))
    return err
}

func (h *customTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    out := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
    out = append(out, h.attrs...)
    out = append(out, attrs...)
    return &customTextHandler{out: h.out, minLevel: h.minLevel, attrs: out, groups: h.groups, timeLayout: h.timeLayout}
}

func (h *customTextHandler) WithGroup(name string) slog.Handler {
    if name == "" {
        return h
    }
    gs := make([]string, 0, len(h.groups)+1)
    gs = append(gs, h.groups...)
    gs = append(gs, name)
    return &customTextHandler{out: h.out, minLevel: h.minLevel, attrs: h.attrs, groups: gs, timeLayout: h.timeLayout}
}

// Setup configures slog's default logger based on provided level and format.
// level: "debug", "info", "warn", "error" (case-insensitive)
// json: if true, use JSON handler; otherwise, use text handler.
// For text logs, time is prefixed as "YYYY/MM/DD HH:MM:SS" without a key.
func Setup(level string, json bool) *slog.Logger {
    var lvl slog.Level
    switch strings.ToLower(level) {
    case "debug":
        lvl = slog.LevelDebug
    case "warn", "warning":
        lvl = slog.LevelWarn
    case "error":
        lvl = slog.LevelError
    default:
        lvl = slog.LevelInfo
    }

    if json {
        // JSON handler with formatted time value.
        replace := func(groups []string, a slog.Attr) slog.Attr {
            if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
                t := a.Value.Time()
                return slog.String(slog.TimeKey, t.Format(timeLayout))
            }
            return a
        }
        opts := &slog.HandlerOptions{Level: lvl, ReplaceAttr: replace}
        logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
        slog.SetDefault(logger)
        return logger
    }

    // Custom text handler: timestamp prefix and key=value list
    th := &customTextHandler{
        out:        os.Stdout,
        minLevel:   lvl,
        attrs:      nil,
        groups:     nil,
        timeLayout: timeLayout,
    }
    logger := slog.New(th)
    slog.SetDefault(logger)
    return logger
}

// SetupFromEnv is retained for completeness but not used by main.
func SetupFromEnv() *slog.Logger {
    level := os.Getenv("LOG_LEVEL")
    if level == "" {
        level = "info"
    }
    format := strings.ToLower(os.Getenv("LOG_FORMAT"))
    useJSON := format == "json"
    return Setup(level, useJSON)
}
