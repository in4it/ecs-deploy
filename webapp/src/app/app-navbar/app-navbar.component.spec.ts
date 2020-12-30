import { ComponentFixture, TestBed, waitForAsync } from '@angular/core/testing';

import { AppNavbarComponent } from './app-navbar.component';

describe('AppNavbarComponent', () => {
  let component: AppNavbarComponent;
  let fixture: ComponentFixture<AppNavbarComponent>;

  beforeEach(waitForAsync(() => {
    TestBed.configureTestingModule({
      declarations: [ AppNavbarComponent ]
    })
    .compileComponents();
  }));

  beforeEach(() => {
    fixture = TestBed.createComponent(AppNavbarComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
