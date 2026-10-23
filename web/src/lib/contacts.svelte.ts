import { api, type Alert, type ClientConfig, type Contact, type Kind, type Position, type Status } from "./api";
import { searchTerms } from "./airlines";

class ContactStore {
  config = $state<ClientConfig | null>(null);
  contacts = $state<Contact[]>([]);
  status = $state<Status | null>(null);
  alerts = $state<Alert[]>([]);
  watch = $state<string[]>([]);
  selectedId = $state<string | null>(null);
  track = $state<Position[]>([]);
  hideGround = $state(true);
  kind = $state<"all" | Kind>("all");
  query = $state("");
  error = $state<string | null>(null);
  now = $state(Date.now());

  selected = $derived(this.contacts.find((c) => c.id === this.selectedId) ?? null);

  visible = $derived.by(() => {
    const terms = searchTerms(this.query);
    return this.contacts.filter((c) => {
      if (this.kind !== "all" && c.kind !== this.kind) return false;
      if (this.hideGround && c.on_ground) return false;
      if (terms.length === 0) return true;
      const fields = [c.label, c.reg, c.type, c.id].map((v) => v?.toUpperCase() ?? "");
      return terms.some((t) => fields.some((f) => f.includes(t)));
    });
  });

  /** 0..1 freshness used for icon fade, 1 when just seen, 0 at expiry. */
  freshness(c: Contact): number {
    const expiry = (c.kind === "vessel" ? this.config?.vessel_expiry_s : this.config?.aircraft_expiry_s) ?? 60;
    const age = (this.now - Date.parse(c.seen)) / 1000;
    return Math.max(0, Math.min(1, 1 - age / expiry));
  }

  async start() {
    this.config = await api.config();
    const tick = async () => {
      try {
        const [list, status, alerts, watch] = await Promise.all([
          api.contacts(),
          api.status(),
          api.alerts(),
          api.watchList(),
        ]);
        this.contacts = list.contacts;
        this.status = status;
        this.alerts = alerts.alerts;
        this.watch = watch.watch;
        this.now = Date.now();
        this.error = null;
        if (this.selectedId) {
          if (this.contacts.some((c) => c.id === this.selectedId)) await this.refreshTrack();
          else this.select(null);
        }
      } catch (e) {
        this.error = (e as Error).message;
      }
    };
    await tick();
    setInterval(tick, (this.config?.poll_interval_s ?? 5) * 1000);
    setInterval(() => (this.now = Date.now()), 1000);
  }

  /** Watch-list keys are bare hex codes and MMSIs, lower-case. */
  isWatched(id: string): boolean {
    return this.watch.includes(id.slice(2).toLowerCase());
  }

  async toggleWatch(id: string) {
    const key = id.slice(2).toLowerCase();
    if (this.isWatched(id)) {
      this.watch = this.watch.filter((w) => w !== key);
      await api.unwatch(id).catch(() => undefined);
    } else {
      this.watch = [...this.watch, key];
      await api.watch(id).catch(() => undefined);
    }
  }

  async dismissAlert(a: Alert) {
    this.alerts = this.alerts.filter((x) => x !== a);
    await api.dismissAlert(a).catch(() => undefined);
  }

  async clearAlerts() {
    this.alerts = [];
    await api.clearAlerts().catch(() => undefined);
  }

  async select(id: string | null) {
    this.selectedId = id;
    this.track = [];
    if (id) await this.refreshTrack();
  }

  private async refreshTrack() {
    if (!this.selectedId) return;
    try {
      this.track = (await api.track(this.selectedId)).positions;
    } catch {
      this.track = [];
    }
  }
}

export const store = new ContactStore();
