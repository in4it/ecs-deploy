import { ComponentFixture, TestBed, waitForAsync } from '@angular/core/testing';

import { ServiceDetailsComponent } from './service-details.component';

describe('ServiceDetailsComponent', () => {
  let component: ServiceDetailsComponent;
  let fixture: ComponentFixture<ServiceDetailsComponent>;

  beforeEach(waitForAsync(() => {
    TestBed.configureTestingModule({
      declarations: [ ServiceDetailsComponent ]
    })
    .compileComponents();
  }));

  beforeEach(() => {
    fixture = TestBed.createComponent(ServiceDetailsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
