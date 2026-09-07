# SFS-DBAAS

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square)

This Helm Chart is used by the self-service ArgoCDs of Superphénix to create managed PostgreSQL databases.

Databases are managed by [CloudNativePG](https://cloudnative-pg.io/), which runs on the
virtualization cluster of each AZ. Each database is a `Cluster` in the project's namespace, made of
one primary and zero or more streaming replicas.

> [!important]
> This chart assumes the CloudNativePG operator and its CRDs are already installed on the target
> cluster. Installing them is **not** part of this chart, and is not yet wired into
> `superphenix-system` either.

### Resource names

Resources have a unique ID (see [this](#creating-a-new-resource)) and a name.

Names do not have to be unique across resources and can be identical. They are simply used as a tag to be filtered and found more easily.

To set a name on a resource, use the `.name` value.

> [!note]
> Names **must** begin with a letter or number, and may contain letters, numbers, hyphens, dots, and underscores, up to 63 characters each.

### Setting a resource location

Resources are located within a specific AZ (or *availability zone*). AZs are represented by a unique code.

To deploy your resource in a specific AZ, simply use the code in the `.location` value of the resource.

### Documentation

- [Creating a database](docs/user/create.md)
- [Connecting to a database](docs/user/connect.md)
- [Resizing and scaling](docs/user/scale.md)
- [Exposing a database](docs/user/expose.md)

---

## Values

<h3>Databases</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>databaseDefaults</td>
			<td>object</td>
			<td><pre lang="json">
{
  "imageRepository": "ghcr.io/cloudnative-pg/postgresql",
  "port": 5432
}
</pre>
</td>
			<td>Defaults applied to every database that does not set the value itself.</td>
		</tr>
		<tr>
			<td>databaseDefaults.imageRepository</td>
			<td>string</td>
			<td><pre lang="json">
"ghcr.io/cloudnative-pg/postgresql"
</pre>
</td>
			<td>Container image repository used to run PostgreSQL. The major version is appended as the tag when a database does not pin an explicit `image`.</td>
		</tr>
		<tr>
			<td>databaseDefaults.port</td>
			<td>int</td>
			<td><pre lang="json">
5432
</pre>
</td>
			<td>Port PostgreSQL listens on. Changing it is not supported.</td>
		</tr>
		<tr>
			<td>databases</td>
			<td>object</td>
			<td><pre lang="">
{}
</pre>
</td>
			<td>Defines the managed databases to be created. Each key defines a new database, and is its local ID. See values.yaml for the complete syntax.</td>
		</tr>
	</tbody>
</table>
<h3>Organization parameters</h3>
<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>gitops</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Whether this chart is deployed through GitOps or not. This parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>location</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>AZ in which we're deploying this chart. This parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>organizationID</td>
			<td>string</td>
			<td><pre lang="json">
"00000000-0000-0000-0000-000000000000"
</pre>
</td>
			<td>Superphénix organization to which this project belongs, must be a generated UUIDv4. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>organizationName</td>
			<td>string</td>
			<td><pre lang="json">
"null"
</pre>
</td>
			<td>Superphénix organization's friendly name. This parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>projectID</td>
			<td>string</td>
			<td><pre lang="json">
"00000000-0000-0000-0000-000000000000"
</pre>
</td>
			<td>Superphénix project to which this project belongs, must be a project within the organization, must be a generated UUIDv4. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
		</tr>
		<tr>
			<td>projectName</td>
			<td>string</td>
			<td><pre lang="json">
"null"
</pre>
</td>
			<td>Superphénix project's friendly name. This ID must come from the SPX API, this parameter cannot be overriden by the user.</td>
		</tr>
	</tbody>
</table>

