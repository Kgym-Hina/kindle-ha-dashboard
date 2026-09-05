export type DashboardMode = "portable" | "zone";

export interface DashboardTarget {
  mode: DashboardMode;
  zone_id: string | null;
  width: number;
  height: number;
  background: string;
}

export interface DashboardFrame {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface DashboardStyle {
  color?: string;
  fill?: string;
  stroke?: string;
  border_width?: number;
  radius?: number;
  font_size?: number;
  font_weight?: "normal" | "bold";
  align?: "left" | "center" | "right";
  [key: string]: unknown;
}

export interface DashboardAction {
  type: "navigate_page" | "call_service" | "show_message" | "refresh_config" | "exit";
  page_id?: string;
  domain?: string;
  service?: string;
  service_data?: Record<string, unknown>;
  title?: string;
  message?: string;
  timeout_ms?: number;
  [key: string]: unknown;
}

export interface DashboardElement {
  id: string;
  type: "text" | "button" | "image_button" | "image" | "rect" | "line";
  frame: DashboardFrame;
  style?: DashboardStyle;
  text?: string;
  image?: { src: string; fit?: "contain" | "cover" | "stretch" };
  action?: DashboardAction;
}

export interface DashboardPage {
  id: string;
  name: string;
  background?: string;
  elements: DashboardElement[];
}

export interface DashboardDocument {
  schema: "kindle-dashboard/v1";
  revision: number;
  updated_at?: string;
  target: DashboardTarget;
  pages: DashboardPage[];
}

export interface ZoneInfo {
  entity_id: string;
  zone_id: string;
  name: string;
  state: string;
}

export interface EntitySummary {
  entity_id: string;
  state: string;
  name: string;
  domain: string;
}

export interface KindleMessage {
  target_device_id?: string;
  title: string;
  message: string;
  timeout_ms?: number;
}

export interface RuntimeConfig {
  ha_url: string;
  ha_configured: boolean;
  ha_reachable: boolean;
  auth_mode: "supervisor" | "token" | "missing";
  token_configured: boolean;
}

export const canvasWidth = 600;
export const canvasHeight = 800;
