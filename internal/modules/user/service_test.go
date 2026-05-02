package user

import (
	"context"
	"errors"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fakes
// ─────────────────────────────────────────────────────────────────────────────

type fakeRepo struct {
	users     map[uuid.UUID]*domain.User
	updateErr error
	createErr error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{users: map[uuid.UUID]*domain.User{}} }

func (r *fakeRepo) FindAll(page, limit int, search string) ([]domain.User, int64, error) {
	out := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		if search == "" || strings.Contains(u.Name, search) || strings.Contains(u.Email, search) {
			out = append(out, *u)
		}
	}
	return out, int64(len(out)), nil
}

func (r *fakeRepo) FindByID(id uuid.UUID) (*domain.User, error) {
	if u, ok := r.users[id]; ok {
		copy := *u
		return &copy, nil
	}
	return nil, domain.ErrNotFound
}

func (r *fakeRepo) FindByEmail(email string) (*domain.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			copy := *u
			return &copy, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeRepo) Create(u *domain.User) error {
	if r.createErr != nil {
		return r.createErr
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	copy := *u
	r.users[u.ID] = &copy
	return nil
}

func (r *fakeRepo) Update(u *domain.User) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	copy := *u
	r.users[u.ID] = &copy
	return nil
}

func (r *fakeRepo) Delete(id uuid.UUID) error {
	delete(r.users, id)
	return nil
}

type fakeStorage struct {
	objects map[string]struct{}
	moveErr error
}

func newFakeStorage() *fakeStorage { return &fakeStorage{objects: map[string]struct{}{}} }

func (s *fakeStorage) put(key string) { s.objects[key] = struct{}{} }
func (s *fakeStorage) has(key string) bool {
	_, ok := s.objects[key]
	return ok
}

func (s *fakeStorage) Upload(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	key := folder + "/upload-" + uuid.New().String() + ".png"
	s.put(key)
	return key, nil
}

func (s *fakeStorage) PresignedURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	if objectKey == "" {
		return "", nil
	}
	return "https://example.test/" + objectKey + "?sig=x", nil
}

func (s *fakeStorage) Move(ctx context.Context, src, dst string) error {
	if s.moveErr != nil {
		return s.moveErr
	}
	if !s.has(src) {
		return errors.New("src not found")
	}
	delete(s.objects, src)
	s.put(dst)
	return nil
}

func (s *fakeStorage) Delete(ctx context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

func newSvc() (*fakeRepo, *fakeStorage, Service) {
	repo := newFakeRepo()
	st := newFakeStorage()
	return repo, st, NewService(repo, st)
}

func TestService_Create_PromotesPhoto(t *testing.T) {
	repo, st, svc := newSvc()
	st.put("temp/users/avatar-abcd.png")

	resp, err := svc.Create(context.Background(), dto.CreateUserRequest{
		Name: "John", Email: "john@example.com", Password: "secret123",
		Photo: "temp/users/avatar-abcd.png",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(resp.Photo, "users/") {
		t.Errorf("photo not promoted: %q", resp.Photo)
	}
	if st.has("temp/users/avatar-abcd.png") {
		t.Error("temp object should be moved")
	}
	if !st.has(resp.Photo) {
		t.Error("permanent object should exist")
	}
	if resp.PhotoURL == "" {
		t.Error("expected presigned URL")
	}
	if _, ok := repo.users[resp.ID]; !ok {
		t.Error("user not persisted")
	}
}

func TestService_Create_RollbackPhotoOnDBError(t *testing.T) {
	repo, st, svc := newSvc()
	st.put("temp/users/avatar-abcd.png")
	repo.createErr = errors.New("db down")

	_, err := svc.Create(context.Background(), dto.CreateUserRequest{
		Name: "John", Email: "john@example.com", Password: "secret123",
		Photo: "temp/users/avatar-abcd.png",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	for k := range st.objects {
		if strings.HasPrefix(k, "users/") {
			t.Errorf("promoted object %q should have been rolled back", k)
		}
	}
}

func TestService_Create_DuplicateEmail(t *testing.T) {
	repo, _, svc := newSvc()
	repo.users[uuid.New()] = &domain.User{Email: "dup@example.com", Name: "X", Password: "x", Role: "user"}

	_, err := svc.Create(context.Background(), dto.CreateUserRequest{
		Name: "Y", Email: "dup@example.com", Password: "secret123",
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestService_Update_PromotesAndDeletesOldPhoto(t *testing.T) {
	repo, st, svc := newSvc()
	id := uuid.New()
	st.put("users/" + id.String() + "/old.png")
	repo.users[id] = &domain.User{
		ID: id, Name: "Old", Email: "old@example.com", Password: "x", Role: "user",
		Photo: "users/" + id.String() + "/old.png",
	}
	st.put("temp/users/avatar-new.png")

	resp, err := svc.Update(context.Background(), id, dto.UpdateUserRequest{
		Name: "New", Photo: "temp/users/avatar-new.png",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if resp.Name != "New" {
		t.Errorf("name = %q", resp.Name)
	}
	if !strings.HasPrefix(resp.Photo, "users/") || !strings.HasSuffix(resp.Photo, "avatar-new.png") {
		t.Errorf("photo = %q", resp.Photo)
	}
	if st.has("users/" + id.String() + "/old.png") {
		t.Error("old photo should be deleted")
	}
	if st.has("temp/users/avatar-new.png") {
		t.Error("temp photo should be moved")
	}
}

func TestService_Update_RollbackPhotoOnDBError(t *testing.T) {
	repo, st, svc := newSvc()
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "X", Email: "x@example.com", Password: "p", Role: "user"}
	st.put("temp/users/avatar-new.png")
	repo.updateErr = errors.New("db down")

	_, err := svc.Update(context.Background(), id, dto.UpdateUserRequest{
		Photo: "temp/users/avatar-new.png",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	for k := range st.objects {
		if strings.HasPrefix(k, "users/") {
			t.Errorf("promoted object %q should have been rolled back", k)
		}
	}
}

func TestService_DeletePhoto(t *testing.T) {
	repo, st, svc := newSvc()
	id := uuid.New()
	key := "users/" + id.String() + "/x.png"
	st.put(key)
	repo.users[id] = &domain.User{ID: id, Name: "X", Email: "x@example.com", Password: "p", Role: "user", Photo: key}

	resp, err := svc.DeletePhoto(context.Background(), id)
	if err != nil {
		t.Fatalf("delete photo: %v", err)
	}
	if resp.Photo != "" {
		t.Errorf("photo = %q, want empty", resp.Photo)
	}
	if st.has(key) {
		t.Error("storage object should be removed")
	}
	if repo.users[id].Photo != "" {
		t.Error("repo user photo should be cleared")
	}
}

func TestService_List_DefaultsAndSearch(t *testing.T) {
	repo, _, svc := newSvc()
	for i := 0; i < 3; i++ {
		uid := uuid.New()
		repo.users[uid] = &domain.User{ID: uid, Name: "User", Email: "u@example.com", Role: "user", Password: "x"}
	}

	out, meta, err := svc.List(context.Background(), 0, 0, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 3 {
		t.Errorf("got %d users", len(out))
	}
	if meta.Page != 1 || meta.Limit != 10 {
		t.Errorf("defaults wrong: page=%d limit=%d", meta.Page, meta.Limit)
	}
}
