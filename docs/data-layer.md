# Data layer

Ce guide detaille le pattern Repository et l'acces aux donnees dans Helix. Il couvre le contrat generique du package `data`, l'adaptateur GORM, les filtres portables, la pagination, les requetes generees avec `query:"auto"` et les transactions.

## Sommaire

- [Modele mental](#modele-mental)
- [Repository generique](#repository-generique)
- [Adaptateur GORM](#adaptateur-gorm)
- [Filtres et pagination](#filtres-et-pagination)
- [Requetes generees](#requetes-generees)
- [Transactions](#transactions)
- [Tests](#tests)
- [Erreurs frequentes](#erreurs-frequentes)

## Modele mental

Le package `data` expose des interfaces ORM-neutral. Votre code applicatif depend de `data.Repository[T, ID, TX]`, `data.Filter` et `data.TransactionManager[TX]`. L'adaptateur concret vit dans un package dedie, par exemple `data/gorm`, qui est la seule frontiere autorisee pour importer `gorm.io/gorm`.

`T` est le type d'entite, `ID` est le type de cle primaire, et `TX` est le type de transaction de l'adaptateur. Pour GORM, `TX` vaut `*gorm.DB`.

```go
type User struct {
	ID    int
	Name  string
	Email string
}

type UserRepository interface {
	data.Repository[User, int, *gorm.DB]
}
```

Le marqueur `helix.Repository` reste utile dans vos composants applicatifs pour annoncer l'intention au framework, tandis que `data.Repository[T, ID, TX]` porte le contrat de persistence type.

## Repository generique

Le contrat actuel prend toujours un `context.Context`. Ce contexte transporte l'annulation, les deadlines et, si besoin, la transaction active.

```go
type Repository[T any, ID any, TX any] interface {
	FindAll(ctx context.Context) ([]T, error)
	FindByID(ctx context.Context, id ID) (*T, error)
	FindWhere(ctx context.Context, filter data.Filter) ([]T, error)
	Save(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id ID) error
	Paginate(ctx context.Context, page, size int) (data.Page[T], error)
	WithTransaction(tx data.Transaction[TX]) data.Repository[T, ID, TX]
}
```

Les methodes ont des responsabilites volontairement petites :

- `FindAll(ctx)` retourne tous les enregistrements.
- `FindByID(ctx, id)` retourne un pointeur vers l'entite ou une erreur compatible avec `data.ErrRecordNotFound`.
- `FindWhere(ctx, filter)` applique un `data.Filter` portable.
- `Save(ctx, entity)` cree ou met a jour l'entite selon la semantique de l'adaptateur.
- `Delete(ctx, id)` supprime l'enregistrement correspondant a l'identifiant.
- `Paginate(ctx, page, size)` retourne une page et le total.
- `WithTransaction(tx)` retourne un repository lie a une transaction explicite.

Les erreurs publiques principales sont `data.ErrRecordNotFound`, `data.ErrDuplicateKey` et `data.ErrInvalidFilter`. Comme les adaptateurs ajoutent du contexte avec wrapping, testez les erreurs avec `errors.Is`.

```go
user, err := repo.FindByID(ctx, 42)
if errors.Is(err, data.ErrRecordNotFound) {
	return nil
}
if err != nil {
	return err
}
_ = user
```

## Adaptateur GORM

L'adaptateur GORM est dans `data/gorm`. Dans les exemples, l'alias `datagorm` evite la collision avec le package officiel GORM.

```go
import datagorm "github.com/enokdev/helix/data/gorm"

db, err := datagorm.OpenSQLite("file:app.db?cache=shared")
if err != nil {
	return err
}
defer db.Close()

if err := db.AutoMigrate(&User{}); err != nil {
	return err
}
```

`datagorm.OpenSQLite` ouvre une base SQLite avec `TranslateError` active. Le wrapper retourne par `Components()` les composants utiles a enregistrer dans le container : le handle GORM interne, le `*datagorm.TransactionManager` et le wrapper `*datagorm.DB`.

```go
for _, component := range db.Components() {
	if err := container.Register(component); err != nil {
		return err
	}
}
```

Pour un repository simple, utilisez `datagorm.NewRepository`.

```go
repo := datagorm.NewRepository[User, int](gormDB)

users, err := repo.FindAll(ctx)
user, err := repo.FindByID(ctx, 1)
err = repo.Save(ctx, &User{Name: "Ada", Email: "ada@example.test"})
err = repo.Delete(ctx, 1)
```

Le parametre `gormDB` est un `*gorm.DB`. Il doit rester confine au code d'assemblage ou au package `data/gorm`; le reste de l'application peut dependre du contrat `data.Repository[User, int, *gorm.DB]`.

**Note CGO** : l'adaptateur SQLite (`github.com/mattn/go-sqlite3`) requiert CGO. Compilez les tests ou applications SQLite avec `CGO_ENABLED=1` et un compilateur C disponible dans votre `PATH`.

## Filtres et pagination

`data.Filter` permet de construire des conditions sans SQL brut.

Les types centraux sont `data.Filter`, `data.NewFilter` et `data.Condition`.

```go
filter, err := data.NewFilter(
	data.LogicalAnd,
	data.Condition{Field: "Email", Operator: data.OperatorEqual, Value: email},
	data.Condition{Field: "Age", Operator: data.OperatorGreaterThanOrEqual, Value: 18},
)
if err != nil {
	return err
}

users, err := repo.FindWhere(ctx, filter)
```

Les operateurs supportes sont :

- `data.OperatorEqual`
- `data.OperatorNotEqual`
- `data.OperatorGreaterThan`
- `data.OperatorGreaterThanOrEqual`
- `data.OperatorLessThan`
- `data.OperatorLessThanOrEqual`
- `data.OperatorContains`
- `data.OperatorIn`
- `data.OperatorIsNull`
- `data.OperatorIsNotNull`

`data.LogicalAnd` exige que toutes les conditions correspondent. `data.LogicalOr` exige au moins une condition. La logique vide (`data.LogicalDefault`) se comporte comme un `AND` dans l'adaptateur GORM.

La validation refuse les champs vides, les operateurs inconnus et les valeurs nil quand l'operateur attend une valeur. `data.OperatorIn` exige une slice ou un array non vide.

```go
page, err := repo.Paginate(ctx, 2, 20)
if err != nil {
	return err
}

fmt.Println(page.Items)
fmt.Println(page.Total)
fmt.Println(page.Page)
fmt.Println(page.PageSize)
```

`data.Page[T]` contient `Items []T`, `Total int`, `Page int` et `PageSize int`. `Paginate(page, size)` refuse les valeurs inferieures a 1 et plafonne la taille (`size`) a 1000 pour preserver la stabilite.

## Requetes generees
Le generateur sait produire des implementations GORM pour des methodes de repository taguees `query:"auto"`. Les methodes doivent recevoir `context.Context` en premier parametre.

```go
type UserRepository interface {
	data.Repository[User, int, *gorm.DB]

	FindByEmail(ctx context.Context, email string) (*User, error) `query:"auto"`
	FindByNameContaining(ctx context.Context, name string) ([]User, error) `query:"auto"`
	FindByAgeGreaterThan(ctx context.Context, age int) ([]User, error) `query:"auto"`
	FindByEmailAndAge(ctx context.Context, email string, age int) (*User, error) `query:"auto"`
	FindAllOrderByCreatedAtDesc(ctx context.Context) ([]User, error) `query:"auto"`
}
```

Les conventions actuellement supportees couvrent :

- `FindByEmail` pour une egalite simple.
- `FindByNameContaining` pour une recherche `LIKE` avec echappement adapte.
- `FindByAgeGreaterThan` pour une comparaison stricte.
- `FindByEmailAndAge` pour combiner plusieurs predicates avec `AND`.
- `FindAllOrderByCreatedAtDesc` ou `FindAllOrderByCreatedAtAsc` pour trier tous les resultats.

`Containing` exige un parametre `string`. Les methodes qui retournent un seul element doivent retourner `*T`; les methodes multi-resultats doivent retourner `[]T`. `FindAllOrderBy...` ne prend que `context.Context`.

Les fichiers generes commencent par :

```go
// Code generated by helix generate. DO NOT EDIT.
```

Ne modifiez pas ces fichiers a la main. Corrigez l'interface source, puis relancez `helix generate`.

## Transactions

Le package `data` modelise les transactions sans exposer l'ORM dans tout le framework.

Les types centraux sont `data.Transaction[TX]` et `data.TransactionManager[TX]`.

```go
type Transaction[TX any] interface {
	Unwrap() TX
}

type TransactionManager[TX any] interface {
	WithinTransaction(ctx context.Context, fn func(context.Context, data.Transaction[TX]) error) error
}
```

Avec GORM :

```go
manager := datagorm.NewTransactionManager(gormDB)

err := manager.WithinTransaction(ctx, func(txCtx context.Context, tx data.Transaction[*gorm.DB]) error {
	txRepo := repo.WithTransaction(tx)
	return txRepo.Save(txCtx, &user)
})
```

`datagorm.NewTransactionManager` construit le manager GORM. `data.ContextWithTransaction` place une transaction dans un contexte enfant. `data.TransactionFromContext` la recupere par type `TX`. Les repositories GORM consultent ce contexte automatiquement, ce qui permet a un repository normal de rejoindre une transaction deja active.

```go
txCtx, err := data.ContextWithTransaction[*gorm.DB](ctx, tx)
if err != nil {
	return err
}

if active, ok := data.TransactionFromContext[*gorm.DB](txCtx); ok {
	_ = active
}
```

Pour le chemin declaratif, annotez une methode de service avec `//helix:transactional`.

```go
type UserService struct {
	helix.Service
	Repo data.Repository[User, int, *gorm.DB] `inject:"true"`
}

//helix:transactional
func (s *UserService) CreateUser(ctx context.Context, user User) error {
	return s.Repo.Save(ctx, &user)
}
```

Le format est strict : `//helix:transactional`, sans espace apres `//`, sans prefixe `+` et sans option supplementaire. La methode doit avoir un receiver, recevoir `context.Context` en premier parametre et retourner `error` en dernier resultat.

Le generateur cree un wrapper du type `NewUserServiceTransactional(target, txManager)`. Le wrapper demarre une transaction via `data.TransactionManager[*gorm.DB]`, commit si l'erreur finale est nil, rollback si elle est non nil, et rejoint la transaction existante quand le contexte en contient deja une. C'est une propagation REQUIRED.

## Tests

Pour tester le contrat portable, utilisez des tests unitaires co-localises autour du package qui depend de `data.Repository`. Pour l'adaptateur GORM, les tests d'integration du framework utilisent SQLite in-memory avec le build tag `integration`.

```bash
go test ./data/...
CGO_ENABLED=1 go test -tags integration ./data/gorm/...
```

Les tests de requetes generees doivent verifier l'interface source et le comportement du fichier produit par `helix generate`, jamais modifier directement un fichier `*_gen.go`.

## Erreurs frequentes

**Oublier `context.Context`** : les methodes du repository et les methodes `query:"auto"` attendent `context.Context` en premier parametre.

**Utiliser l'ancienne forme generique** : le contrat courant est `data.Repository[T, ID, TX]`, pas `data.Repository[T, ID]`.

**Exposer GORM partout** : gardez `gorm.io/gorm` dans le code d'assemblage, les repositories GORM ou les signatures transactionnelles necessaires. Le code metier devrait privilegier les interfaces `data`.

**Paginer avec des valeurs invalides** : `Paginate(0, 20)` et `Paginate(1, 0)` retournent une erreur.

**Passer une valeur scalaire a `OperatorIn`** : utilisez une slice ou un array non vide.

**Ajouter des options a `//helix:transactional`** : `//helix:transactional readOnly` est invalide avec le generateur actuel.
