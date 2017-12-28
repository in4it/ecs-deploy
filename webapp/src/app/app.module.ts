import { BrowserModule } from '@angular/platform-browser';
import { NgModule } from '@angular/core';
import { FormsModule }    from '@angular/forms';

import { ReactiveFormsModule } from '@angular/forms';

import { AppComponent } from './app.component';
import { AppNavbarComponent } from './app-navbar/app-navbar.component';

import { NgbModule } from '@ng-bootstrap/ng-bootstrap';
import { DeploymentListComponent } from './deployment-list/deployment-list.component';
import { DeploymentListResolver }   from './deployment-list/deployment-list-resolver.service';
import { DeploymentListService }   from './deployment-list/deployment-list.service';

import {HttpClientModule, HTTP_INTERCEPTORS } from '@angular/common/http';

import { RouterModule, Routes } from '@angular/router';
import { LoginComponent } from './login/login.component';
import { LoginSAMLComponent } from './login/saml.component';
import { AlertService, AuthService } from './services/index';
import { AuthGuard } from './guards/auth.guard';
import { AlertComponent } from './directives/alert.component';

import { AppHttpInterceptor } from './interceptors/http-interceptor';
import { ErrorComponent } from './errors/error.component';
import { ServiceListComponent } from './service-list/service-list.component';
import { ServiceListResolver }   from './service-list/service-list-resolver.service';
import { ServiceListService }   from './service-list/service-list.service';
import { ServiceDetailComponent } from './service-detail/service-detail.component';
import { ServiceDetailResolver } from './service-detail/service-detail-resolver.service';
import { ServiceDetailService } from './service-detail/service-detail.service';
import { InspectChildComponent } from './service-detail/inspect.component';
import { DeployChildComponent } from './service-detail/deploy.component';
import { ConfirmChildComponent } from './service-detail/confirm.component';

// routes
const appRoutes: Routes = [
  { path: 'login', component: LoginComponent },
  { path: 'saml', component: LoginSAMLComponent },
  {
    path: 'deployments',
    component: DeploymentListComponent,
    resolve: { dl: DeploymentListResolver },
    data: { title: 'ECS Deploy tool' },
    canActivate: [AuthGuard]
  },
  {
    path: 'services',
    component: ServiceListComponent,
    resolve: { sl: ServiceListResolver },
    data: { title: 'ECS Deploy tool' },
    canActivate: [AuthGuard]
  },
  {
    path: 'service/:serviceName',
    component: ServiceDetailComponent,
    resolve: { sd: ServiceDetailResolver },
    data: { title: 'ECS Deploy tool' },
    canActivate: [AuthGuard]
  },
  { path: '',
    redirectTo: '/services',
    pathMatch: 'full'
  },
  { path: 'error', component: ErrorComponent },
  { path: '**', data: { error: 'PageNotFound' }, component: ErrorComponent }
];


@NgModule({
  declarations: [
    AppComponent,
    AlertComponent,
    AppNavbarComponent,
    DeploymentListComponent,
    LoginComponent,
    LoginSAMLComponent,
    ErrorComponent,
    ServiceListComponent,
    ServiceDetailComponent,
    InspectChildComponent,
    DeployChildComponent,
    ConfirmChildComponent,
  ],
  imports: [
    BrowserModule,
    ReactiveFormsModule,
    HttpClientModule,
    FormsModule,
    RouterModule.forRoot(
      appRoutes,
      { enableTracing: false } // <-- debugging purposes only
    ),
    NgbModule.forRoot()
  ],
  providers: [
    AuthGuard,
    AlertService,
    AuthService,
    DeploymentListService,
    DeploymentListResolver,
    ServiceListResolver,
    ServiceListService,
    ServiceDetailResolver,
    ServiceDetailService,
    { provide: HTTP_INTERCEPTORS, useClass: AppHttpInterceptor, multi: true },
  ],
  bootstrap: [AppComponent]
})

export class AppModule { }
