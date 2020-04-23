# Team Onboarding

This examples showcases a possible onboarding process for a team. We assume the following:

1. Teams are identified by a groups in an external Identity Provider System (such as LDAP).  
2. There is a group sync job that between LDAP and OCP
3. Groups marked with a label `type: devteam` need to be on-boarded. (this assumption is needed to showcase some feature and not really required, another option can be that all synched teams need to be on-boarded).

We have the following requirements:

1. Each team will get 4 namespaces: `<team>-build`, `<team>-dev`, `<team>-qa` and `<team>-prod`.
2. These four projects receive a multiproject quota, it is up to the team to manage it.
3. Builds can occur only in the `<team>-project`.
4. Each project will have automatically assigned egress IPs (we assume the [egressip-ipam-operator]() is installed).
5. Build projects can communicate only with a set of predefined endpoints (some of them out of the corporate network), but cannot communicate with the corporate network.
6. Run projects can communicate only with the corporate network (represented by this CIDR: `10.20.0.0/0`), with the exclusion of the OCP nodes (represented by this CIDR `10.20.32.0/0`).
7. By default each project cannot communicate with other projects in teh cluster, but the team is given the ability to manage their own network policies.

For this scenario we will need to configure several resources. Let's start from the UserConfig:


