package acme.admin;

public class BulkExport {
  private final Exporter exporter;
  private final Audit audit;
  public BulkExport(Exporter exporter, Audit audit) {
    this.exporter = exporter;
    this.audit = audit;
  }
  private void audited(Request req, Runnable action) {
    audit.record(req.actor(), "export", req.filter());
    action.run();
  }
  public void run(Request req) {
    audited(req, () -> exporter.export(req.filter()));
  }
}
