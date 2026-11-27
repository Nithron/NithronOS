// Network and Remote Access Types

export type AccessMode = 'lan_only' | 'wireguard' | 'public_https';
// 'dns_01' isn't offered: it needs Caddy plugins Debian's caddy lacks.
export type HTTPSMode = 'self_signed' | 'http_01';

export interface NetworkStatus {
  access_mode: AccessMode;
  lan_access: boolean;
  wan_blocked: boolean;
  wireguard?: WireGuardConfig;
  https?: HTTPSConfig;
  firewall?: FirewallState;
  external_ip?: string;
  internal_ips?: string[];
  open_ports?: number[];
}

export interface FirewallState {
  mode: AccessMode;
  rules: FirewallRule[];
  last_applied: string;
  checksum: string;
  status: 'active' | 'pending_confirm' | 'rolling_back';
  rollback_at?: string;
}

export interface FirewallRule {
  id: string;
  table: string;
  chain: string;
  priority: number;
  type: 'allow' | 'deny' | 'nat';
  protocol?: string;
  source_cidr?: string;
  dest_port?: string;
  action: string;
  description: string;
  enabled: boolean;
}

export interface FirewallPlan {
  id: string;
  current_state?: FirewallState;
  desired_state?: FirewallState;
  changes: FirewallDiff[];
  dry_run_output: string;
  created_at: string;
  expires_at: string;
}

export interface FirewallDiff {
  type: 'add' | 'remove' | 'modify';
  rule?: FirewallRule;
  old_rule?: FirewallRule;
  description: string;
}

export interface WireGuardConfig {
  enabled: boolean;
  interface: string;
  public_key: string;
  listen_port: number;
  server_cidr: string;
  endpoint_hostname: string;
  dns?: string[];
  peers: WireGuardPeer[];
  last_handshake?: string;
  bytes_rx: number;
  bytes_tx: number;
}

export interface WireGuardPeer {
  id: string;
  name: string;
  public_key: string;
  allowed_ips: string[];
  endpoint?: string;
  last_handshake?: string;
  bytes_rx: number;
  bytes_tx: number;
  created_at: string;
  enabled: boolean;
}

export interface WireGuardPeerConfig {
  interface: {
    private_key: string;
    address: string;
    dns?: string[];
  };
  peer: {
    public_key: string;
    preshared_key?: string;
    endpoint: string;
    allowed_ips: string[];
    persistent_keepalive?: number;
  };
  qr_code: string;
  config: string;
}

export interface HTTPSConfig {
  mode: HTTPSMode;
  domain?: string;
  email?: string;
  dns_provider?: string;
  cert_path?: string;
  key_path?: string;
  status: 'pending' | 'active' | 'failed' | 'renewing';
  expiry?: string;
  last_renewal?: string;
  next_renewal?: string;
  error_message?: string;
}

export interface TOTPEnrollment {
  secret: string;
  qr_code: string;
  backup_codes: string[];
  uri: string;
}

export interface TOTPStatus {
  enrolled: boolean;
  verified: boolean;
  required: boolean;
}

export interface RemoteAccessWizardState {
  step: number;
  access_mode: AccessMode;
  wireguard_config?: WireGuardConfig;
  https_config?: HTTPSConfig;
  firewall_plan?: FirewallPlan;
  completed: boolean;
  error?: string;
}

// Request types
export interface EnableWireGuardRequest {
  cidr: string;
  listen_port?: number;
  endpoint_hostname?: string;
  dns?: string[];
}

export interface AddWireGuardPeerRequest {
  name: string;
  allowed_ips?: string[];
  public_key?: string;
}

export interface ConfigureHTTPSRequest {
  mode: HTTPSMode;
  domain?: string;
  email?: string;
  dns_provider?: string;
  dns_api_key?: string;
}

export interface PlanFirewallRequest {
  desired_mode: AccessMode;
  enable_wg: boolean;
  enable_https: boolean;
  custom_rules?: FirewallRule[];
}

export interface ApplyFirewallRequest {
  plan_id: string;
  rollback_timeout_sec?: number;
}

export interface VerifyTOTPRequest {
  code: string;
}

export interface EnrollTOTPRequest {
  password: string;
}
