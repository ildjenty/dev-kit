package acme.admin;

public interface Audit { void record(String actor, String action, String filter); }
