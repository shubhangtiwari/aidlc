# Fixture Architecture

The fixture has a small Go CLI, an internal core package, an auth package, and a reusable util
package.

## Layers

Internal packages own application behavior. Package util exposes shared helpers.
