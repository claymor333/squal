---
name: laravel-conventions
description: "Project-specific Laravel, PHP, and Docker conventions. Activates when writing or editing PHP code, creating migrations, controllers, models, seeders, running artisan commands, using Pint, Pest, or discussing Laravel architecture, database patterns, Herd, or Gitea PRs."
license: MIT
metadata:
  author: project
---

# Laravel Conventions

Laravel, PHP, and project-specific conventions for this application.

## Foundational Context

- php - 8.4.19
- laravel/framework (LARAVEL) - v12
- laravel/prompts (PROMPTS) - v0
- laravel/sanctum (SANCTUM) - v4
- livewire/livewire (LIVEWIRE) - v4
- tightenco/ziggy (ZIGGY) - v2
- laravel/mcp (MCP) - v0
- laravel/pint (PINT) - v1
- laravel/sail (SAIL) - v1
- pestphp/pest (PEST) - v3
- phpunit/phpunit (PHPUNIT) - v11
- alpinejs (ALPINEJS) - v3
- prettier (PRETTIER) - v3

## Docker PHP Execution

- ALL PHP commands MUST be run inside the `php84` Docker container using `docker exec`.
- Use: `docker exec php84 <command>` — for example:
  - `docker exec php84 php artisan migrate --no-interaction`
  - `docker exec php84 php artisan test --compact`
  - `docker exec php84 vendor/bin/pint --dirty --format agent`
  - `docker exec php84 composer install`
- Never run `php`, `composer`, `artisan`, `phpunit`, `pest`, or `pint` directly on the host.
- **Artisan Commands via Laravel Boost MCP:** Whenever you need to run `php artisan` commands (e.g., migrations, test creation, model generation), prefer using the Laravel Boost MCP tools instead of `docker exec php84 php artisan`. Use the `list-artisan-commands` tool to check available commands, and use `search-docs` for version-specific documentation.

## Skills Activation

- `pest-testing` — Tests applications using the Pest 3 PHP framework. Activates when writing tests, creating unit or feature tests, adding assertions, testing Livewire components, architecture testing, debugging test failures, working with datasets or mocking; or when the user mentions test, spec, TDD, expects, assertion, coverage, or needs to verify functionality works.
- `blaze-optimize` — Set up and optimize Blade component rendering with Blaze. Use when installing Blaze, optimizing components, or configuring @blaze directives and strategies.

## Laravel Boost Rules

- Laravel Boost is an MCP server that comes with powerful tools designed specifically for this application. Use them.
- Use the `list-artisan-commands` tool when you need to call an Artisan command to double-check the available parameters.
- **Always use Laravel Boost MCP tools for artisan commands** instead of `docker exec php84 php artisan`.
- Whenever you share a project URL with the user, use the `get-absolute-url` tool to ensure correct scheme, domain/IP, and port.
- Use the `tinker` tool when you need to execute PHP to debug code or query Eloquent models directly.
- Use the `database-query` tool when you only need to read from the database.
- Use the `database-schema` tool to inspect table structure before writing migrations or models.
- You can read browser logs, errors, and exceptions using the `browser-logs` tool from Boost. Only recent browser logs will be useful.
- Search the documentation before making code changes to ensure we are taking the correct approach.
- Use multiple, broad, simple, topic-based queries at once. For example: `['rate limiting', 'routing rate limiting', 'routing']`.
- Do not add package names to queries; package information is already shared.

### Available Search Syntax

1. Simple Word Searches with auto-stemming - query=authentication - finds 'authenticate' and 'auth'.
2. Multiple Words (AND Logic) - query=rate limit - finds knowledge containing both "rate" AND "limit".
3. Quoted Phrases (Exact Position) - query="infinite scroll" - words must be adjacent and in that order.
4. Mixed Queries - query=middleware "rate limit" - "middleware" AND exact phrase "rate limit".
5. Multiple Queries - queries=["authentication", "middleware"] - ANY of these terms.

## PHP Rules

- Always use curly braces for control structures, even for single-line bodies.
- Use PHP 8 constructor property promotion in `__construct()`.
    - `public function __construct(public GitHub $github) { }`
- Do not allow empty `__construct()` methods with zero parameters unless the constructor is private.
- Always use explicit return type declarations for methods and functions.
- Use appropriate PHP type hints for method parameters.
- Typically, keys in an Enum should be TitleCase. For example: `FavoritePerson`, `BestLake`, `Monthly`.
- Prefer PHPDoc blocks over inline comments. Never use comments within the code itself unless the logic is exceptionally complex.
- Add useful array shape type definitions when appropriate.

## Herd Rules

- The application is served by Laravel Herd and will be available at: `https?://[kebab-case-project-dir].test`. Use the `get-absolute-url` tool to generate valid URLs for the user.
- You must not run any commands to make the site available via HTTP(S). It is always available through Laravel Herd.
- Local URL: `http://myprojek2.local` (no SSL).
- SSO login can be bypassed by navigating to `/login-as/{users.id}`. Use this in E2E tests and manual testing instead of going through the SSO flow.

## Test Enforcement

- Every change must be programmatically tested. Write a new test or update an existing test, then run the affected tests to make sure they pass.
- Run the minimum number of tests needed to ensure code quality and speed. Use `php artisan test --compact` with a specific filename or filter.

## Laravel Core Rules

- Use `php artisan make:` commands to create new files (i.e. migrations, controllers, models, etc.). You can list available Artisan commands using the `list-artisan-commands` tool.
- **Always use Laravel Boost MCP tools for artisan commands** instead of `docker exec php84 php artisan`.
- If you're creating a generic PHP class, use `php artisan make:class`.
- Pass `--no-interaction` to all Artisan commands to ensure they work without user input. You should also pass the correct `--options` to ensure correct behavior.

### Database

- Always use proper Eloquent relationship methods with return type hints. Prefer relationship methods over raw queries or manual joins.
- Use Eloquent models and relationships before suggesting raw database queries.
- Avoid `DB::`; prefer `Model::query()`. Generate code that leverages Laravel's ORM capabilities rather than bypassing them.
- Generate code that prevents N+1 query problems by using eager loading.
- Use Laravel's query builder for very complex database operations.

### Model Creation

- When creating new models, create useful factories and seeders for them too. Ask the user if they need any other things, using `list-artisan-commands` to check the available options to `php artisan make:model`.

### APIs & Eloquent Resources

- For APIs, default to using Eloquent API Resources and API versioning unless existing API routes do not, then you should follow existing application convention.

### Controllers & Validation

- Always create Form Request classes for validation rather than inline validation in controllers. Include both validation rules and custom error messages.
- Check sibling Form Requests to see if the application uses array or string based validation rules.

### Authentication & Authorization

- Use Laravel's built-in authentication and authorization features (gates, policies, Sanctum, etc.).

### URL Generation

- When generating links to other pages, prefer named routes and the `route()` function.

### Queues

- Use queued jobs for time-consuming operations with the `ShouldQueue` interface.

### Configuration

- Use environment variables only in configuration files - never use the `env()` function directly outside of config files. Always use `config('app.name')`, not `env('APP_NAME')`.

### Testing

- When creating models for tests, use the factories for the models. Check if the factory has custom states that can be used before manually setting up the model.
- Faker: Use methods such as `$this->faker->word()` or `fake()->randomDigit()`. Follow existing conventions whether to use `$this->faker` or `fake()`.
- When creating tests, make use of `php artisan make:test [options] {name}` to create a feature test, and pass `--unit` to create a unit test. Most tests should be feature tests.

### Vite Error

- If you receive an "Illuminate\Foundation\ViteException: Unable to locate file in Vite manifest" error, you can run `npm run build` or ask the user to run `npm run dev` or `composer run dev`.

## Laravel v12 Rules

- CRITICAL: ALWAYS use `search-docs` tool for version-specific Laravel documentation and updated code examples.
- This project upgraded from Laravel 10 without migrating to the new streamlined Laravel file structure.
- This is perfectly fine and recommended by Laravel. Follow the existing structure from Laravel 10. We do not need to migrate to the new Laravel structure unless the user explicitly requests it.

### Laravel 10 Structure

- Middleware typically lives in `app/Http/Middleware/` and service providers in `app/Providers/`.
- There is no `bootstrap/app.php` application configuration in a Laravel 10 structure:
    - Middleware registration happens in `app/Http/Kernel.php`
    - Exception handling is in `app/Exceptions/Handler.php`
    - Console commands and schedule register in `app/Console/Kernel.php`
    - Rate limits likely exist in `RouteServiceProvider` or `app/Http/Kernel.php`

### Database (v12)

- When modifying a column, the migration must include all of the attributes that were previously defined on the column. Otherwise, they will be dropped and lost.
- Laravel 12 allows limiting eagerly loaded records natively, without external packages: `$query->latest()->limit(10);`.

### Models (v12)

- Casts can and likely should be set in a `casts()` method on a model rather than the `$casts` property. Follow existing conventions from other models.

### Migrations & Seeders

- Migrations are for schema only: create/alter/drop tables, columns, indexes, foreign keys. Never put data inserts in a migration.
- Any data insert/update (reference & lookup rows, roles, abilities, menus, statuses, email templates, config rows) must go into a seeder, not a migration.
- Put seeders in `database/seeders/`; module-specific seeders go in `database/seeders/myprojek2/`. Register new ones in `DatabaseSeeder` (or `Myprojek2Seeder` for module seeders) so they run with `db:seed`.
- Seeders must be idempotent — use `updateOrCreate()` / `firstOrCreate()` / `upsert()` so re-running is safe and does not duplicate rows.
- Only exception: a data backfill that a schema change depends on (e.g. filling a new `NOT NULL` column from existing rows) may stay in the migration. That means transforming existing rows — still never inserting new reference data.

## Pint Rules

- You must run `vendor/bin/pint --dirty --format agent` before finalizing changes to ensure your code matches the project's expected style.
- **NEVER run pint without the `--dirty` and `--format agent` flags.** Always use: `docker exec php84 vendor/bin/pint --dirty --format agent`
- Do not run `vendor/bin/pint --test --format agent`, simply run `vendor/bin/pint --format agent` to fix any formatting issues.

## Pest Rules

- This project uses Pest for testing. Create tests: `php artisan make:test --pest {name}`.
- Run tests: `php artisan test --compact` or filter: `php artisan test --compact --filter=testName`.
- Do NOT delete tests without approval.
- CRITICAL: ALWAYS use `search-docs` tool for version-specific Pest documentation and updated code examples.
- IMPORTANT: Activate `pest-testing` every time you're working with a Pest or testing-related task.
