package acme.admin;

public record Request(String actor, String filter, boolean fast) {}
