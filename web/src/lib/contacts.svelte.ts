import { api, type ClientConfig, type Contact, type Position, type Status } from "./api";

class ContactStore {
  config = $state<ClientConfig | null>(null);
  contacts = $state<Contact[]>([]);
  status = $state<Status | null>(null);
  selectedId = $state<string | null>(null);
  track = $state<Position[]>([]);
  hideGround = $state(true);
  query = $state("");
  error = $state<string | null>(null);
  now = $state(Date.now());

  selected = $derived(this.contacts.find((c) => c.id === this.selectedId) ?? null);

  visible = $derived.by(() => {
    const q = this.query.trim().toLowerCase();
    return this.contacts.filter((c) => {
      if (this.hideGround && c.on_ground) return false;
      if (!q) return true;
      return [c.label, c.reg, c.type, c.id].some((v) => v?.toLowerCase().includes(q));
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
        const [list, status] = await Promise.all([api.contacts(), api.status()]);
        this.contacts = list.contacts;
        this.status = status;
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
