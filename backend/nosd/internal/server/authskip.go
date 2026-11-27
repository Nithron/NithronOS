package server

import "sync/atomic"

// skipAuthChecks turns off the sign-in, admin and CSRF checks when the
// router is built. Only this package's tests set it (TestMain, and
// withRealAuth to turn the checks back on): no setting, environment
// variable or file can disable authentication on a real system.
var skipAuthChecks atomic.Bool
